package isession

import (
	"errors"
	"sync"

	"github.com/ooclab/es/common"
)

type Pool struct {
	curID     uint32
	idMutex   *sync.Mutex
	pool      map[uint32]*Session
	poolMutex *sync.Mutex
}

func newPool() *Pool {
	return &Pool{
		curID:     0,
		idMutex:   &sync.Mutex{},
		pool:      map[uint32]*Session{},
		poolMutex: &sync.Mutex{},
	}
}

func (p *Pool) newID() uint32 {
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

func (p *Pool) Exist(id uint32) bool {
	p.poolMutex.Lock()
	_, exist := p.pool[id]
	p.poolMutex.Unlock()
	return exist
}

func (p *Pool) New(outbound chan *common.LinkOMSG) (*Session, error) {
	id := p.newID()

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	session := newSession(id, outbound)
	p.pool[id] = session

	return session, nil
}

func (p *Pool) Get(id uint32) *Session {
	p.poolMutex.Lock()
	v, exist := p.pool[id]
	p.poolMutex.Unlock()
	if exist {
		return v
	} else {
		return nil
	}
}

func (p *Pool) Delete(session *Session) error {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[session.ID]
	if !exist {
		return errors.New("delete failed: session not exist")
	}
	delete(p.pool, session.ID)
	return nil
}
