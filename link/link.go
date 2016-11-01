package link

import (
	"io"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/common"
	"github.com/ooclab/es/etp"
	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/tunnel"
)

type LinkConfig struct {
}

type Link struct {
	ID string // uuid ?

	isessionManager *isession.Manager
	tunnelManager   *tunnel.Manager

	outbound         chan *common.LinkOMSG
	connDisconnected chan bool
}

func NewLink(config *LinkConfig) *Link {
	l := &Link{
		outbound: make(chan *common.LinkOMSG, 1),
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

	c := newConn(conn)

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

			// dispatch
			switch m.Type {

			case common.LinkMsgTypeInnerSession:
				err = l.isessionManager.HandleIn(m.Payload)

			case common.LinkMsgTypeTunnel:
				err = l.tunnelManager.HandleIn(m.Payload)

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
				Version: common.DefaultLinkMSGVersion,
				Type:    omsg.Type,
				Flag:    common.DefaultLinkMSGFlag,
				Payload: omsg.Payload,
			}

			if err := c.Send(m); err != nil {
				logrus.Errorf("send message to link conn failed: %s", err)
				break
			}
		}
	}()

	return nil
}

func (l *Link) Close() error {
	logrus.Warn("l.Close() is not completed!")
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
