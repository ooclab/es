package tunnel

import (
	"errors"
	"sync"
)

type Pool struct {
	curID     uint32
	idMutex   *sync.Mutex
	pool      map[uint32]*Tunnel
	poolMutex *sync.Mutex
}

func NewPool() *Pool {
	return &Pool{
		curID:     0,
		idMutex:   &sync.Mutex{},
		pool:      map[uint32]*Tunnel{},
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

func (p *Pool) Get(id uint32) *Tunnel {
	p.poolMutex.Lock()
	v, exist := p.pool[id]
	p.poolMutex.Unlock()
	if exist {
		return v
	} else {
		return nil
	}
}

func (p *Pool) Delete(t *Tunnel) error {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[t.ID]
	if !exist {
		return errors.New("delete failed: tunnel not exist")
	}
	delete(p.pool, t.ID)
	return nil
}

func (p *Pool) New(config *TunnelConfig) (*Tunnel, error) {
	id := p.newID()

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	t := newTunnel(id, config)
	p.pool[id] = t

	return t, nil
}
