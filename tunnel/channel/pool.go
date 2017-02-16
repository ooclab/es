package channel

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

type Pool struct {
	nextID    uint32
	pool      map[uint32]*Channel
	poolMutex sync.RWMutex
}

func NewPool() *Pool {
	return &Pool{
		nextID:    1,
		pool:      map[uint32]*Channel{},
		poolMutex: sync.RWMutex{},
	}
}

func (p *Pool) newID() (id uint32) {
	for {
		id = atomic.AddUint32(&p.nextID, 1)
		if !p.Exist(id) {
			return
		}
	}
}

func (p *Pool) Exist(id uint32) bool {
	p.poolMutex.RLock()
	_, exist := p.pool[id]
	p.poolMutex.RUnlock()
	return exist
}

func (p *Pool) Get(id uint32) *Channel {
	p.poolMutex.Lock()
	v, exist := p.pool[id]
	p.poolMutex.Unlock()
	if exist {
		return v
	} else {
		return nil
	}
}

func (p *Pool) Delete(c *Channel) error {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[c.ChannelID]
	if !exist {
		return errors.New("delete failed: channel not exist")
	}
	delete(p.pool, c.ChannelID)
	return nil
}

func (p *Pool) New(tid uint32, outbound chan []byte, conn net.Conn) *Channel {
	cid := p.newID()
	return p.NewByID(cid, tid, outbound, conn)
}

func (p *Pool) NewByID(cid uint32, tid uint32, outbound chan []byte, conn net.Conn) *Channel {
	p.poolMutex.Lock()
	c := &Channel{
		TunnelID:  tid,
		ChannelID: cid,
		Outbound:  outbound,
		Conn:      conn,
		lock:      &sync.Mutex{},
	}
	p.pool[cid] = c
	p.poolMutex.Unlock()
	return c
}
