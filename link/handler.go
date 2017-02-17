package link

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/tunnel"
)

type requestHandler struct {
	router *isession.Router
}

func newRequestHandler(routes []isession.Route) *requestHandler {
	h := &requestHandler{
		router: isession.NewRouter(),
	}
	h.router.AddRoutes([]isession.Route{
		{"/echo", h.echo},
	})
	h.router.AddRoutes(routes)
	return h
}

func (h *requestHandler) Handle(m *isession.EMSG) *isession.EMSG {
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

	return &isession.EMSG{
		Type:    isession.MsgTypeResponse,
		ID:      m.ID,
		Payload: payload,
	}
}

func (h *requestHandler) echo(req *isession.Request) (*isession.Response, error) {
	return &isession.Response{Status: "success", Body: req.Body}, nil
}

func defaultTunnelCreateHandler(manager *tunnel.Manager) isession.RequestHandlerFunc {
	return func(r *isession.Request) (resp *isession.Response, err error) {
		cfg := &tunnel.TunnelConfig{}
		if err = json.Unmarshal(r.Body, &cfg); err != nil {
			logrus.Errorf("tunnel create: unmarshal tunnel config failed: %s", err)
			resp.Status = "load-tunnel-map-error"
			return
		}

		logrus.Debug("handle got: ", cfg)

		t, err := manager.TunnelCreate(cfg)
		if err != nil {
			logrus.Errorf("create tunnel failed: %s", err)
			resp.Status = "create-tunnel-failed"
			return
		}

		body, _ := json.Marshal(tunnelCreateBody{ID: t.ID})
		resp = &isession.Response{
			Status: "success",
			Body:   body,
		}
		return
	}
}
