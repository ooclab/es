package link

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/etp"
	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/tunnel"
	"github.com/satori/go.uuid"
)

type LinkEvent struct {
	Link    *Link
	Name    string
	Data    interface{}
	Created time.Time
}

type LinkConfig struct {
}

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
	l := &Link{
		outbound:           make(chan *common.LinkOMSG, 1),
		heartbeatInterval:  3,
		offlineDetectRelay: 15,
		quit:               make(chan bool, 1),
	}
	l.isessionManager = isession.NewManager(l.outbound)
	l.tunnelManager = tunnel.NewManager(l.outbound, l.isessionManager)
	hdr := newRequestHandler([]etp.Route{
		{"POST", "/tunnel", l.tunnelManager.HandleTunnelCreate},
	})
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
		defer conn.Close()
		defer close(l.connDisconnected)

		logrus.Debug("start handler for link messgae recv")
		for {
			m, err := c.Recv()
			if err != nil {
				if err == io.EOF {
					logrus.Debug("conn is closed")
					break
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
		logrus.Debug("HandleIn is stoped")
	}()

	go func() {
		logrus.Debug("start handler for link messgae send")
		for {
			omsg := <-l.outbound
			m := &linkMSG{
				Type:    omsg.Type,
				Payload: omsg.Payload,
			}

			if err := c.Send(m); err != nil {
				logrus.Errorf("send message to link conn failed: %s", err)
				break
			}
		}
		// TODO: quit !
		closeBoolChan(l.quit)
	}()

	return nil
}

func (l *Link) Close() error {
	logrus.Warn("l.Close() is not completed!")
	closeBoolChan(l.quit)
	l.tunnelManager.Close()
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
	// l.lastRecvTimeMutex.Lock()
	// defer l.lastRecvTimeMutex.Unlock()
	l.lastRecvTime = time.Now()
}

func (l *Link) unfreshRecvTime() float64 {
	// l.lastRecvTimeMutex.Lock()
	// defer l.lastRecvTimeMutex.Unlock()
	return time.Since(l.lastRecvTime).Seconds()
}

func (l *Link) SetOffline() {
	logrus.Debugf("link %s is offline", l.ID)
	now := time.Now()
	l.offline = true
	l.offlineTime = now

	event := &LinkEvent{
		Link:    l,
		Name:    "offline",
		Data:    map[string]string{},
		Created: now,
	}
	l.syncSendEvent(event)
}

func (l *Link) SetOnline() {
	logrus.Debugf("link %s is online", l.ID)
	now := time.Now()
	l.offline = false

	event := &LinkEvent{
		Link:    l,
		Name:    "online",
		Data:    map[string]string{},
		Created: now,
	}
	l.syncSendEvent(event)
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
	select {
	case l.Event <- event: // Put event in the channel unless it is full
		return nil
	default:
		logrus.Warn("l.Event Channel full. Discarding value")
		return errors.New("outbound channel is full")
	}
}

func closeBoolChan(b chan bool) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("closeBoolChan recovered: ", r)
		}
	}()
	close(b)
}
