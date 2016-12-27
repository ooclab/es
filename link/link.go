package link

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/tunnel"
	"github.com/satori/go.uuid"
)

var (
	// ErrEventChannelIsFull the event channel is full
	ErrEventChannelIsFull = errors.New("event channel is full")
)

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
}

// Link master connection between two point
type Link struct {
	ID string // uuid ?

	isessionManager *isession.Manager
	tunnelManager   *tunnel.Manager

	outbound         chan *common.LinkOMSG
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
}

func NewLink(config *LinkConfig) *Link {
	return newLink(config, nil)
}

func NewLinkCustom(config *LinkConfig, hdr isession.RequestHandler) *Link {
	return newLink(config, hdr)
}

func newLink(config *LinkConfig, hdr isession.RequestHandler) *Link {
	l := &Link{
		outbound:           make(chan *common.LinkOMSG, 1),
		heartbeatInterval:  15,
		offlineDetectRelay: 60,
		lastRecvTimeMutex:  &sync.Mutex{},
		lastRecvTime:       time.Now(), // FIXME: init time to prevent SetOffline triggered when program started
		quit:               make(chan bool, 1),
	}
	if config == nil {
		config = &LinkConfig{}
	}
	l.isessionManager = isession.NewManager(config.IsServerSide, l.outbound)
	l.tunnelManager = tunnel.NewManager(config.IsServerSide, l.outbound, l.isessionManager)
	if hdr == nil {
		hdr = newRequestHandler([]isession.Route{
			{"/tunnel", l.tunnelManager.HandleTunnelCreate},
		})
	}
	l.isessionManager.SetRequestHandler(hdr)
	return l
}

func (l *Link) Bind(conn io.ReadWriteCloser) error {
	l.connDisconnected = make(chan bool, 1)

	l.ID = uuid.NewV4().String()

	c := newConn(conn)

	// heartbeat
	go func() {
		for {
			select {
			case <-time.After(time.Second * 3):

				unfresh := l.unfreshRecvTime()
				// fmt.Println("-- ", time.Now(), l.lastRecvTime, l.offline, unfresh, l.offlineDetectRelay)

				if l.offline {
					if unfresh < float64(l.offlineDetectRelay) {
						l.SetOnline()
					}
				} else {
					if unfresh >= float64(l.offlineDetectRelay) {
						l.SetOffline()
					}
				}

				if unfresh >= float64(l.heartbeatInterval) {
					// send heartbeat
					// fmt.Println("start sendHeartbeatRequest ...")
					if err := l.sendHeartbeatRequest(); err != nil {
						logrus.Debug("send heartbeat request failed: ", err)
					}
					// fmt.Println("finish sendHeartbeatRequest ...")
				}

			case <-l.quit:
				logrus.Debugf("link %s: heartbeat is quit", l.ID)
				return
			}
		}
	}()

	go func() {
		// TODO:
		defer safeRun(func() { conn.Close() })
		defer closeBoolChan(l.connDisconnected)
		defer closeBoolChan(l.quit)

		logrus.Debug("start handler for link message recv")
		for {
			m, err := c.Recv()
			if err != nil {
				if err == io.EOF {
					logrus.Debug("conn is closed")
					break
				}

				if err == ErrMessageLengthTooLarge {
					logrus.Warnf("link %s got a large message, CLOSE IT.", l.ID)
					// TODO: notice remote endpoint!
				}

				// TODO
				logrus.Errorf("read link frame error: %s", err)
				break
			}

			l.updateLastRecvTime()

			// dispatch
			switch m.Type {

			case common.LinkMsgTypeInnerSession:
				err = l.isessionManager.HandleIn(m.Payload)

			case common.LinkMsgTypeTunnel:
				err = l.tunnelManager.HandleIn(m.Payload)

			case common.LinkMsgTypeHeartbeatRequest:
				l.sendHeartbeatResponse()

			case common.LinkMsgTypeHeartbeatResponse:
				logrus.Debugf("link %s heartbeat success", l.ID)

			default:
				logrus.Errorf("unknown link message: %s", m)
				err = ErrMessageTypeUnknown

			}

			if err != nil {
				logrus.Errorf("handle link message %s error: %s", m, err)
				break
			}
		}
		logrus.Debugf("link %s: HandleIn is stoped", l.ID)
	}()

	go func() {
		var omsg *common.LinkOMSG
		for {
			select {
			case omsg = <-l.outbound:
				if omsg == nil {
					logrus.Errorf("link %s: l.outbound is closed", l.ID)
					closeBoolChan(l.quit)
					return
				}
			case <-l.quit:
				logrus.Debugf("link %s: got l.quit event,  quit recv l.outbound", l.ID)
				return
			}
			m := &linkMSG{
				Type:    omsg.Type,
				Payload: omsg.Payload,
			}

			if err := c.Send(m); err != nil {
				logrus.Errorf("link %s: send message to conn failed: %s", l.ID, err)
				safeRun(func() { conn.Close() })
				closeBoolChan(l.connDisconnected)
				closeBoolChan(l.quit)
				return
			}
		}
	}()

	return nil
}

func (l *Link) closeOutbound() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("closeOutbound recovered: ", r)
		}
	}()
	close(l.outbound)
}

func (l *Link) Close() error {
	closeBoolChan(l.quit)
	l.closeOutbound()
	l.tunnelManager.Close()
	l.isessionManager.Close()
	return nil
}

func (l *Link) WaitDisconnected() error {
	<-l.connDisconnected
	logrus.Debugf("link %s is disconnected", l.ID)
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

func (l *Link) IsClosed() bool {
	return l.closed
}

// OfflineDuration get the duration time of the offline
func (l *Link) OfflineDuration() time.Duration {
	return time.Since(l.offlineTime)
}

func (l *Link) SetHeartbeatInterval(seconds int) {
	l.heartbeatInterval = seconds
}

func (l *Link) sendHeartbeatRequest() error {
	m := &common.LinkOMSG{
		Type: common.LinkMsgTypeHeartbeatRequest,
		// Payload: []byte{},
	}
	return l.syncSend(m)
}

func (l *Link) sendHeartbeatResponse() error {
	m := &common.LinkOMSG{
		Type: common.LinkMsgTypeHeartbeatResponse,
		// Payload: []byte{},
	}
	return l.syncSend(m)
}

func (l *Link) syncSend(m *common.LinkOMSG) error {
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

func safeRun(f func()) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("safeRun recovered: ", r)
		}
	}()
	f()
}

func closeBoolChan(b chan bool) {
	safeRun(func() {
		close(b)
	})
}
