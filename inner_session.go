package es

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/labstack/gommon/log"
	"github.com/ooclab/es/etp"
)

type InnerSession struct {
	ID       uint32
	response chan []byte
	link     *Link
}

func newInnerSession(id uint32, link *Link) *InnerSession {
	return &InnerSession{
		ID:       id,
		response: make(chan []byte, 1),
		link:     link,
	}
}

func (session *InnerSession) HandleResponse(payload []byte) error {
	logrus.Debugf("inner session : got response : %s", string(payload))
	session.response <- payload
	return nil
}

func (session *InnerSession) request(payload []byte) (respPayload []byte, err error) {
	// TODO:

	err = session.link.writeFrame(LINK_FRAME_TYPE_INNERSESSION_REQ, session.ID, payload)
	if err != nil {
		return nil, err
	}

	// TODO: timeout
	respPayload = <-session.response
	return respPayload, nil
}

func (session *InnerSession) Post(url string, body []byte) (resp *etp.Response, err error) {
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

type InnerSessionPool struct {
	curID     uint32
	idMutex   *sync.Mutex
	pool      map[uint32]*InnerSession
	poolMutex *sync.Mutex
}

func newInnerSessionPool() *InnerSessionPool {
	return &InnerSessionPool{
		curID:     0,
		idMutex:   &sync.Mutex{},
		pool:      map[uint32]*InnerSession{},
		poolMutex: &sync.Mutex{},
	}
}

func (p *InnerSessionPool) newID() uint32 {
	p.idMutex.Lock()
	defer p.idMutex.Unlock()
	for {
		p.curID++
		if p.curID <= 0 {
			continue
		}
		if !p.Exist(p.curID) {
			break
		}
	}
	return p.curID
}

func (p *InnerSessionPool) Exist(id uint32) bool {
	p.poolMutex.Lock()
	_, exist := p.pool[id]
	p.poolMutex.Unlock()
	return exist
}

func (p *InnerSessionPool) New(link *Link) (*InnerSession, error) {
	id := p.newID()

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	session := newInnerSession(id, link)
	p.pool[id] = session

	return session, nil
}

func (p *InnerSessionPool) Get(id uint32) *InnerSession {
	p.poolMutex.Lock()
	v, exist := p.pool[id]
	p.poolMutex.Unlock()
	if exist {
		return v
	} else {
		return nil
	}
}

func (p *InnerSessionPool) Delete(session *InnerSession) error {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[session.ID]
	if !exist {
		return errors.New("delete failed: session not exist")
	}
	delete(p.pool, session.ID)
	return nil
}

type innerSessionRequestHandler struct {
	router *etp.Router
	link   *Link
}

func newInnerSessionRequestHandler(link *Link) *innerSessionRequestHandler {
	h := &innerSessionRequestHandler{
		router: etp.NewRouter(),
		link:   link,
	}
	h.router.AddRoutes([]etp.Route{
		{"POST", "/echo", h.handleEcho},
	})
	return h
}

func (h *innerSessionRequestHandler) Handle(payload []byte) []byte {
	w := etp.NewResponseWriter()
	req, err := etp.ReadRequest(payload)
	if err != nil {
		w.WriteHeader(etp.StatusBadRequest)
		log.Warn("bad request!")
	} else {
		err = h.router.Dispatch(w, req)
		if err != nil {
			log.Errorf("dispatch internal route (%s:%s) failed: %s\n", req.Method, req.RequestURI, err)
		}
	}

	return w.Bytes()
}

func (h *innerSessionRequestHandler) handleEcho(w etp.ResponseWriter, r *etp.Request) {
	w.Write(r.Body)
}
