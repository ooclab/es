package isession

import (
	"encoding/json"
	"ooclab/emsg/etp"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/emsg"
	"github.com/ooclab/es/tunnel"
)

type requestHandler struct {
	router        *etp.Router
	tunnelManager *tunnel.Manager
}

func newRequestHandler(tm *tunnel.Manager) *requestHandler {
	h := &requestHandler{
		router:        etp.NewRouter(),
		tunnelManager: tm,
	}
	h.router.AddRoutes([]etp.Route{
		{"POST", "/echo", h.echo},
		{"POST", "/tunnel", h.tunnelCreate},
	})
	return h
}

func (h *requestHandler) Handle(m *emsg.EMSG) *emsg.EMSG {
	w := etp.NewResponseWriter()
	req, err := etp.ReadRequest(m.Payload)
	if err != nil {
		w.WriteHeader(etp.StatusBadRequest)
		logrus.Warn("bad request!")
	} else {
		err = h.router.Dispatch(w, req)
		if err != nil {
			logrus.Errorf("%s %s: %s", req.Method, req.RequestURI, err)
		}
	}

	return &emsg.EMSG{
		Type:    MsgTypeResponse,
		ID:      m.ID,
		Payload: w.Bytes(),
	}
}

func (h *requestHandler) echo(w etp.ResponseWriter, r *etp.Request) {
	w.Write(r.Body)
}

func (h *requestHandler) tunnelCreate(w etp.ResponseWriter, r *etp.Request) {
	cfg := &tunnel.TunnelConfig{}
	if err := json.Unmarshal(r.Body, &cfg); err != nil {
		logrus.Errorf("tunnel create: unmarshal tunnel config failed: %s", err)
		etp.WriteJSON(w, etp.StatusExpectationFailed, map[string]string{"error": "load-tunnel-map-error"})
		return
	}

	cfg.Reverse = !cfg.Reverse // FIXME!

	h.tunnelManager.TunnelCreate(cfg)
	// fmt.Println("new tunnel ", tm)
	etp.WriteJSON(w, etp.StatusOK, map[string]interface{}{
		"tunnel_id": 123,
	})
}
