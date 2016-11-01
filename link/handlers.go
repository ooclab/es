package link

import (
	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/emsg"
	"github.com/ooclab/es/etp"
	"github.com/ooclab/es/isession"
)

type requestHandler struct {
	router *etp.Router
}

func newRequestHandler(routes []etp.Route) *requestHandler {
	h := &requestHandler{
		router: etp.NewRouter(),
	}
	h.router.AddRoutes([]etp.Route{
		{"POST", "/echo", h.echo},
		// {"POST", "/tunnel", h.tunnelCreate},
	})
	h.router.AddRoutes(routes)
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
		Type:    isession.MsgTypeResponse,
		ID:      m.ID,
		Payload: w.Bytes(),
	}
}

func (h *requestHandler) echo(w etp.ResponseWriter, r *etp.Request) {
	w.Write(r.Body)
}
