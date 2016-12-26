package link

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/emsg"
	"github.com/ooclab/es/isession"
)

type requestHandler struct {
	router *isession.Router
}

func newRequestHandler(routes []isession.Route) *requestHandler {
	h := &requestHandler{
		router: isession.NewRouter(),
	}
	h.router.AddRoutes(routes)
	return h
}

func (h *requestHandler) Handle(m *emsg.EMSG) *emsg.EMSG {
	var resp *isession.Response
	var err error

	req := &isession.Request{}
	if err = json.Unmarshal(m.Payload, &req); err != nil {
		logrus.Errorf("json unmarshal isession request failed: %s", err)
		resp = &isession.Response{Status: "json-unmarshal-request-error"}
	} else {
		resp, err = h.router.Dispatch(req)
		if err != nil {
			logrus.Errorf("dispatch request failed: %s", err)
			resp = &isession.Response{Status: "dispatch-request-error"}
		}
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		logrus.Errorf("json marshal response failed: %s", err)
		resp = &isession.Response{Status: "json-marshal-response-error"}
	}

	return &emsg.EMSG{
		Type:    isession.MsgTypeResponse,
		ID:      m.ID,
		Payload: payload,
	}
}

func (h *requestHandler) echo(req *isession.Request) (*isession.Response, error) {
	return &isession.Response{Status: "success", Body: req.Body}, nil
}
