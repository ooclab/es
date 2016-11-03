package tunnel

import (
	"errors"
	"fmt"
	"sync"
)

type listenPool struct {
	pool      map[string]Tunneler
	poolMutex *sync.Mutex
}

func newListenPool() *listenPool {
	return &listenPool{
		pool:      map[string]Tunneler{},
		poolMutex: &sync.Mutex{},
	}
}

func (p *listenPool) getKey(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func (p *listenPool) Exist(host string, port int) bool {
	key := p.getKey(host, port)
	p.poolMutex.Lock()
	_, exist := p.pool[key]
	p.poolMutex.Unlock()
	return exist
}

func (p *listenPool) Get(host string, port int) Tunneler {
	key := p.getKey(host, port)
	p.poolMutex.Lock()
	v, exist := p.pool[key]
	p.poolMutex.Unlock()
	if exist {
		return v
	}

	return nil
}

func (p *listenPool) Delete(host string, port int) error {
	key := p.getKey(host, port)
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[key]
	if !exist {
		return errors.New("delete failed: listen port not exist")
	}
	delete(p.pool, key)
	return nil
}
