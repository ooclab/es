package isession

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es"
	"github.com/ooclab/es/emsg"
)

type Manager struct {
	pool           *Pool
	outbound       chan []byte
	requestHandler RequestHandler
}

func NewManager(isServerSide bool, outbound chan []byte) *Manager {
	m := &Manager{
		pool:     newPool(isServerSide),
		outbound: outbound,
	}
	return m
}

func (manager *Manager) SetRequestHandler(hdr RequestHandler) {
	manager.requestHandler = hdr
}

func (manager *Manager) HandleIn(payload []byte) error {
	m, err := emsg.LoadEMSG(payload)
	if err != nil {
		return err
	}

	switch m.Type {

	case MsgTypeRequest:
		rMsg := manager.requestHandler.Handle(m)
		manager.outbound <- append([]byte{es.LinkMsgTypeSession}, rMsg.Bytes()...)

	case MsgTypeResponse:
		s := manager.pool.Get(m.ID)
		if s == nil {
			logrus.Errorf("can not find isession with ID %d", m.ID)
			return errors.New("no such isession")
		}
		s.HandleResponse(m.Payload)

	default:
		logrus.Errorf("unknown isession msg type: %d", m.Type)
		return errors.New("unknown isession msg type")

	}

	return nil
}

func (manager *Manager) New() (*Session, error) {
	return manager.pool.New(manager.outbound)
}

func (manager *Manager) Close() {
	for item := range manager.pool.IterBuffered() {
		item.Val.Close()
		logrus.Debugf("close session %s", item.Key)
		manager.pool.Delete(item.Val)
	}
}

// func (l *Link) openInnerSession() (*InnerSession, error) {
// 	return l.innerSessionPool.New(l)
// }
//
// func (l *Link) handleInnerSessionRequest(frame *linkFrame) error {
// 	payload := l.requestHandler.Handle(frame.Payload)
// 	return l.writeFrame(LINK_FRAME_TYPE_INNERSESSION_REP, frame.ID, payload)
// }
//
// func (l *Link) handleInnerSessionResponse(frame *linkFrame) error {
// 	session := l.innerSessionPool.Get(frame.ID)
// 	if session == nil {
// 		return errors.New("no such inner session")
// 	}
//
// 	return session.HandleResponse(frame.Payload)
// }
//
// func (l *Link) sessionStream(frame *linkFrame) error {
// 	logrus.Infof("got session message: %s", frame)
// 	return nil
// }
