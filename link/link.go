package link

import (
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es"
	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/tunnel"
)

// Define error
var (
	ErrEventChannelIsFull = errors.New("event channel is full")
	ErrLinkShutdown       = errors.New("link is shutdown")
	ErrTimeout            = errors.New("timeout")
	ErrKeepAliveTimeout   = errors.New("keepalive error")
	ErrMsgPingInvalid     = errors.New("invalid ping message")
)

const (
	sizeOfType   = 1
	sizeOfLength = 3
	headerSize   = sizeOfType + sizeOfLength
)

type linkMSGHeader struct {
	Type   uint8
	Length uint32
}

type linkMSG struct {
	Header linkMSGHeader
	Body   io.Reader
}

// LinkEvent the event struct of link
type LinkEvent struct {
	Link    *Link
	Name    string
	Data    interface{}
	Created time.Time
}

// LinkConfig reserved for config
type LinkConfig struct {
	// ID need to be started differently
	IsServerSide bool

	// EnableKeepalive is used to do a period keep alive
	// messages using a ping.
	EnableKeepAlive bool

	// KeepAliveInterval is how often to perform the keep alive
	KeepAliveInterval time.Duration

	// ConnectionWriteTimeout is meant to be a "safety valve" timeout after
	// we which will suspect a problem with the underlying connection and
	// close it. This is only applied to writes, where's there's generally
	// an expectation that things will move along quickly.
	ConnectionWriteTimeout time.Duration
}

// Link master connection between two point
type Link struct {
	ID     uint32
	config *LinkConfig

	isessionManager *isession.Manager
	tunnelManager   *tunnel.Manager

	// bufRead:    bufio.NewReader(conn)
	// recvBuf = bytes.NewBuffer(make([]byte, 0, length))
	outbound         chan []byte
	connDisconnected chan bool

	lastRecvTime      time.Time
	lastRecvTimeMutex *sync.Mutex
	heartbeatInterval int

	Event chan *LinkEvent

	offline            bool
	offlineDetectRelay int
	offlineTime        time.Time
	closed             bool
	quit               chan bool

	// pings is used to track inflight pings
	pings    map[uint32]chan struct{}
	pingID   uint32
	pingLock sync.Mutex

	// shutdown is used to safely close a link
	shutdown     bool
	shutdownErr  error
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewLink create a new link
func NewLink(config *LinkConfig) *Link {
	return newLink(config, nil)
}

// NewLinkCustom create a new link by custom isession.RequestHandler
func NewLinkCustom(config *LinkConfig, hdr isession.RequestHandler) *Link {
	return newLink(config, hdr)
}

func newLink(config *LinkConfig, hdr isession.RequestHandler) *Link {
	if config == nil {
		config = &LinkConfig{
			EnableKeepAlive:        true,
			KeepAliveInterval:      30 * time.Second,
			ConnectionWriteTimeout: 10 * time.Second,
		}
	}
	l := &Link{
		config:             config,
		outbound:           make(chan []byte, 1),
		heartbeatInterval:  15,
		offlineDetectRelay: 60,
		lastRecvTimeMutex:  &sync.Mutex{},
		lastRecvTime:       time.Now(), // FIXME: init time to prevent SetOffline triggered when program started
		quit:               make(chan bool, 1),

		pings:      make(map[uint32]chan struct{}),
		shutdownCh: make(chan struct{}),
	}
	l.isessionManager = isession.NewManager(config.IsServerSide, l.outbound)
	l.tunnelManager = tunnel.NewManager(config.IsServerSide, l.outbound, l.isessionManager)
	if hdr == nil {
		hdr = newRequestHandler([]isession.Route{
			{"/tunnel", l.tunnelManager.HandleTunnelCreate},
		})
	}
	l.isessionManager.SetRequestHandler(hdr)
	if config.EnableKeepAlive {
		go l.keepalive()
	}
	return l
}

// IsClosed does a safe check to see if we have shutdown
func (l *Link) IsClosed() bool {
	select {
	case <-l.shutdownCh:
		return true
	default:
		return false
	}
}

func (l *Link) closeOutbound() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("closeOutbound recovered: ", r)
		}
	}()
	close(l.outbound)
}

// Close is used to close the link
func (l *Link) Close() error {
	l.shutdownLock.Lock()
	defer l.shutdownLock.Unlock()

	if l.shutdown {
		return nil
	}
	l.shutdown = true
	if l.shutdownErr == nil {
		l.shutdownErr = ErrLinkShutdown
	}
	close(l.shutdownCh)
	l.closeOutbound()
	// TODO: close isessions & tunnles
	l.tunnelManager.Close()
	l.isessionManager.Close()
	return nil
}

// exitErr is used to handle an error that is causing the
// link to terminate.
func (l *Link) exitErr(err error) {
	l.shutdownLock.Lock()
	if l.shutdownErr == nil {
		l.shutdownErr = err
	}
	l.shutdownLock.Unlock()
	l.Close()
}

// Ping is used to measure the RTT response time
func (l *Link) Ping() (time.Duration, error) {
	// Get a channel for the ping
	ch := make(chan struct{})

	// Get a new ping id, mark as pending
	l.pingLock.Lock()
	id := l.pingID
	l.pingID++
	l.pings[id] = ch
	l.pingLock.Unlock()

	// Send the ping request
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, id)
	l.outbound <- append([]byte{es.LinkMsgTypePingRequest}, payload...)

	// Wait for a response
	start := time.Now()
	select {
	case <-ch:
	case <-time.After(l.config.ConnectionWriteTimeout):
		l.pingLock.Lock()
		delete(l.pings, id) // Ignore it if a response comes later.
		l.pingLock.Unlock()
		return 0, ErrTimeout
	case <-l.shutdownCh:
		return 0, ErrLinkShutdown
	}

	// Compute the RTT
	return time.Now().Sub(start), nil
}

// keepalive is a long running goroutine that periodically does
// a ping to keep the connection alive.
func (l *Link) keepalive() {
	for {
		select {
		case <-time.After(l.config.KeepAliveInterval):
			_, err := l.Ping()
			if err != nil {
				logrus.Printf("link %d: keepalive failed: %v", l.ID, err)
				l.exitErr(ErrKeepAliveTimeout)
				return
			}
		case <-l.shutdownCh:
			return
		}
	}
}

func (l *Link) recv(conn es.Conn, errCh chan error) {
	logrus.Debug("start handler for link message recv")
	for {
		m, err := conn.Recv()
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "reset by peer") {
				logrus.Errorf("link %d: Failed to read header: %v", l.ID, err)
			}
			// TODO: nil
			errCh <- err
			return
		}

		l.updateLastRecvTime()

		mType, mData := m[0], m[1:]

		// dispatch
		switch mType {
		case es.LinkMsgTypeSession:
			err = l.isessionManager.HandleIn(mData)
		case es.LinkMsgTypeTunnel:
			err = l.tunnelManager.HandleIn(mData)
		case es.LinkMsgTypePingRequest:
			l.outbound <- append([]byte{es.LinkMsgTypePingResponse}, mData...)
		case es.LinkMsgTypePingResponse:
			err = l.handlePing(mData)
		default:
			logrus.Errorf("link %d: unknown message type %d", l.ID, mType)
			// TODO:
			errCh <- errors.New("unknown message type")
			return
		}

		if err != nil {
			logrus.Errorf("link %d: handle link message error: %s", l.ID, err)
			errCh <- err
			return
		}
	}
}

func (l *Link) send(conn es.Conn, errCh chan error) {
	for {
		select {
		case m := <-l.outbound:
			// FIXME!
			if m == nil {
				errCh <- errors.New("get nil from l.outbound")
				return
			}
			err := conn.Send(m)
			if err != nil {
				logrus.Errorf("link %d: write data to conn failed: %s", l.ID, err)
				errCh <- err
				return
			}
		case <-l.shutdownCh:
			logrus.Debugf("link %d: send is stop", l.ID)
			errCh <- nil
			return
		}
	}
}

// Bind bind link with a underlying connection (tcp)
func (l *Link) Bind(conn es.Conn) error {
	errCh := make(chan error, 1)
	go l.recv(conn, errCh)
	go l.send(conn, errCh)
	// wait stop
	return <-errCh
}

// handlePing is invokde for a LinkMsgTypePing frame
func (l *Link) handlePing(payload []byte) error {
	if len(payload) != 4 {
		return ErrMsgPingInvalid
	}
	pingID := binary.LittleEndian.Uint32(payload)
	l.pingLock.Lock()
	ch := l.pings[pingID]
	if ch != nil {
		delete(l.pings, pingID)
		close(ch)
	}
	l.pingLock.Unlock()
	return nil
}

func (l *Link) OpenInnerSession() (*isession.Session, error) {
	return l.isessionManager.New()
}

func (l *Link) OpenTunnel(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) error {
	return l.tunnelManager.OpenTunnel(localHost, localPort, remoteHost, remotePort, reverse)
}

func (l *Link) updateLastRecvTime() {
	l.lastRecvTimeMutex.Lock()
	defer l.lastRecvTimeMutex.Unlock()
	l.lastRecvTime = time.Now()
}

func (l *Link) unfreshRecvTime() float64 {
	l.lastRecvTimeMutex.Lock()
	defer l.lastRecvTimeMutex.Unlock()
	return time.Since(l.lastRecvTime).Seconds()
}

func (l *Link) SetOffline() {
	logrus.Debugf("link %s is offline", l.ID)
	now := time.Now()
	l.offline = true
	l.offlineTime = now

	if l.Event != nil {
		event := &LinkEvent{
			Link:    l,
			Name:    "offline",
			Data:    map[string]string{},
			Created: now,
		}
		l.syncSendEvent(event)
	}
}

func (l *Link) SetOnline() {
	logrus.Debugf("link %s is online", l.ID)
	now := time.Now()
	l.offline = false

	if l.Event != nil {
		event := &LinkEvent{
			Link:    l,
			Name:    "online",
			Data:    map[string]string{},
			Created: now,
		}
		l.syncSendEvent(event)
	}
}

// IsOffline check link status
func (l *Link) IsOffline() bool {
	return l.offline
}

// OfflineDuration get the duration time of the offline
func (l *Link) OfflineDuration() time.Duration {
	return time.Since(l.offlineTime)
}

func (l *Link) syncSend(m []byte) error {
	// fmt.Println("syncSend ...", m)
	select {
	case l.outbound <- m: // Put m in the channel unless it is full
		return nil
	case <-time.After(time.Second * 1):
		// default:
		logrus.Warnf("link %s outbound Channel full. Discarding value", l.ID)
		return errors.New("outbound channel is full")
	}
}

func (l *Link) syncSendEvent(event *LinkEvent) error {
	if l.Event == nil {
		return nil
	}
	select {
	case l.Event <- event: // Put event in the channel unless it is full
		return nil
	default:
		logrus.Debug("l.Event Channel full. Discarding value")
		return ErrEventChannelIsFull
	}
}
