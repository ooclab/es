package tunnel

import (
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/session"
	tcommon "github.com/ooclab/es/tunnel/common"
	"github.com/ooclab/es/util"
)

type Manager struct {
	pool           *Pool
	lpool          *listenPool
	outbound       chan []byte
	sessionManager *session.Manager
}

func NewManager(isServerSide bool, outbound chan []byte, sm *session.Manager) *Manager {
	return &Manager{
		pool:           NewPool(isServerSide),
		lpool:          newListenPool(),
		outbound:       outbound,
		sessionManager: sm,
	}
}

func (manager *Manager) HandleIn(payload []byte) error {
	m, err := tcommon.LoadTMSG(payload)
	if err != nil {
		return err
	}

	switch m.Type {

	case tcommon.MsgTypeChannelForward:
		t := manager.pool.Get(m.TunnelID)
		if t == nil {
			logrus.Warnf("can not find tunnel %d", m.TunnelID)
			return errors.New("can not find tunnel")
		}
		return t.HandleIn(m)

	case tcommon.MsgTypeChannelClose:
		t := manager.pool.Get(m.TunnelID)
		if t == nil {
			logrus.Warnf("can not find tunnel %d", m.TunnelID)
			return errors.New("no such tunnel")
		}
		t.HandleChannelClose(m)
		// return nil

	default:
		logrus.Errorf("unknown tunnel msg type: %d", m.Type)
		return errors.New("unknown tunnel msg type")

	}

	return nil
}

func (manager *Manager) TunnelCreate(cfg *TunnelConfig) (*Tunnel, error) {
	logrus.Debugf("prepare to create a tunnel with config %+v", cfg)
	t, err := manager.pool.New(cfg, manager.outbound)
	if err != nil {
		logrus.Errorf("create new tunnel failed: %s", err)
		return nil, err
	}

	if !cfg.Reverse {
		if err := manager.runListenTunnel(t); err != nil {
			logrus.Errorf("run forward tunnel failed!")
			manager.pool.Delete(t)
			return nil, err
		}
	}

	logrus.Debugf("create forward tunnel: %+v", t)
	return t, nil
}

func (manager *Manager) listenTCP(host string, port int) (net.Listener, error) {

	if manager.lpool.Exist(host, port) {
		// the listen address is exist in lpool already
		logrus.Errorf("start listen for %s:%d failed, it's existed already.", host, port)
		return nil, errors.New("listen address is existed")
	}

	// start listen
	addr := fmt.Sprintf("%s:%d", host, port)
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if nil != err {
		logrus.Fatalln(err)
	}
	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		// the listen address is taken by another program
		logrus.Errorf("start listen on %s failed: %s", addr, err)
		return nil, err
	}

	// save listen
	manager.lpool.Add(host, port, l)
	return l, nil
}

// runListenTunnel the tunnel in listen side
func (manager *Manager) runListenTunnel(t *Tunnel) error {
	cfg := t.Config
	if cfg.Reverse {
		return errors.New("not forward tunnel")
	}
	l, err := manager.listenTCP(cfg.LocalHost, cfg.LocalPort)
	if err != nil {
		return err
	}

	logrus.Debugf("start listen tunnel %s success", t)

	go func() {
		// defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				if util.TCPisClosedConnError(err) {
					logrus.Debugf("the listener of %s is closed", t)
				} else {
					logrus.Errorf("accept new client failed: %s", err)
				}
				break
			}
			logrus.Debugf("accept %s", conn.RemoteAddr())

			c := t.NewChannelByConn(conn)
			go t.ServeChannel(c)
			logrus.Debugf("OPEN channel %s success", c)
		}
	}()

	return nil
}

func (manager *Manager) Close() error {
	for item := range manager.lpool.IterBuffered() {
		fmt.Println("item = ", item)
		item.Val.Close()
		logrus.Debugf("close listen %s", item.Key)
		manager.lpool.Delete(item.Key)
	}
	return nil
}
