package tunnel

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
)

type listenPool struct {
	pool      map[string]net.Listener
	poolMutex *sync.Mutex
}

func newListenPool() *listenPool {
	return &listenPool{
		pool:      map[string]net.Listener{},
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

func (p *listenPool) Add(host string, port int, l net.Listener) {
	key := p.getKey(host, port)
	p.poolMutex.Lock()
	p.pool[key] = l
	p.poolMutex.Unlock()
}

func (p *listenPool) Get(host string, port int) net.Listener {
	key := p.getKey(host, port)
	p.poolMutex.Lock()
	v, exist := p.pool[key]
	p.poolMutex.Unlock()
	if exist {
		return v
	}

	return nil
}

func (p *listenPool) Delete(key string) error {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	_, exist := p.pool[key]
	if !exist {
		return errors.New("delete failed: listen port not exist")
	}
	delete(p.pool, key)
	logrus.Debugf("delete listen %s from pool success", key)
	return nil
}

// Used by the Iter & IterBuffered functions to wrap two variables together over a channel,
type listenPoolTuple struct {
	Key string
	Val net.Listener
}

// Returns a buffered iterator which could be used in a for range loop.
func (p *listenPool) IterBuffered() <-chan listenPoolTuple {
	ch := make(chan listenPoolTuple, len(p.pool))
	go func() {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			// Foreach key, value pair.
			p.poolMutex.Lock()
			defer p.poolMutex.Unlock()
			for key, val := range p.pool {
				ch <- listenPoolTuple{key, val}
			}
			wg.Done()
		}()
		wg.Wait()
		close(ch)
	}()
	return ch
}
