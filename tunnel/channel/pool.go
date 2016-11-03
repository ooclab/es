package channel

import (
	"errors"
	"net"
	"sync"

	"github.com/ooclab/es/common"
)

type Pool struct {
	curID     uint32
	idMutex   *sync.Mutex
	pool      map[uint32]*Channel
	poolMutex *sync.Mutex
}

func NewPool() *Pool {
	return &Pool{
		curID:     0,
		idMutex:   &sync.Mutex{},
		pool:      map[uint32]*Channel{},
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

func (p *Pool) New(tid uint32, outbound chan *common.LinkOMSG, conn net.Conn) *Channel {
	cid := p.newID()
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
