package isession

import (
	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/emsg"
	"github.com/ooclab/es/etp"
)

type Session struct {
	ID       uint32
	inbound  chan []byte
	outbound chan *common.LinkOMSG
}

func newSession(id uint32, outbound chan *common.LinkOMSG) *Session {
	return &Session{
		ID:       id,
		inbound:  make(chan []byte, 1),
		outbound: outbound,
	}
}

func (session *Session) HandleResponse(payload []byte) error {
	logrus.Debugf("inner session : got response : %s", string(payload))
	session.inbound <- payload
	return nil
}

func (session *Session) request(payload []byte) (respPayload []byte, err error) {
	// TODO:

	m := &emsg.EMSG{
		Type:    MsgTypeRequest,
		ID:      session.ID,
		Payload: payload,
	}

	session.outbound <- &common.LinkOMSG{
		Type:    common.LinkMsgTypeInnerSession,
		Payload: m.Bytes(),
	}

	// TODO: timeout
	respPayload = <-session.inbound
	return respPayload, nil
}

func (session *Session) Post(url string, body []byte) (resp *etp.Response, err error) {
	req, err := etp.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	respPayload, err := session.request(req.Bytes())
	if err != nil {
		return nil, err
	}

	return etp.ReadResponse(respPayload)
}
