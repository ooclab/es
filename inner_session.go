package es

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
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

func (session *InnerSession) Request(payload []byte) (respPayload []byte, err error) {
	if len(payload) > int(LINK_FRAME_LENGTH_MAX) {
		return nil, errors.New("request is too large")
	}

	err = session.link.writeFrame(LINK_FRAME_TYPE_INNERSESSION_REQ, session.ID, payload)
	if err != nil {
		return nil, err
	}

	// TODO: timeout
	respPayload = <-session.response
	return respPayload, nil
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
