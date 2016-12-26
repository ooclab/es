package isession

import (
	"encoding/json"

	"github.com/ooclab/es/common"
	"github.com/ooclab/es/emsg"
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

func (session *Session) Close() {
	close(session.inbound)
}

func (session *Session) HandleResponse(payload []byte) error {
	// logrus.Debugf("inner session : got response : %s", string(payload))
	session.inbound <- payload
	return nil
}

func (session *Session) SendAndWait(payload []byte) (respPayload []byte, err error) {
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

func (session *Session) SendJSONAndWait(request interface{}, response interface{}) error {
	reqData, err := json.Marshal(request)
	if err != nil {
		return err
	}
	respData, err := session.SendAndWait(reqData)
	if err != nil {
		return err
	}
	return json.Unmarshal(respData, &response)
}
