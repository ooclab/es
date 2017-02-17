package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/isession"
	tcommon "github.com/ooclab/es/tunnel/common"
	"github.com/ooclab/es/util"
)

type Manager struct {
	pool            *Pool
	lpool           *listenPool
	outbound        chan []byte
	isessionManager *isession.Manager
}

func NewManager(isServerSide bool, outbound chan []byte, ism *isession.Manager) *Manager {
	return &Manager{
		pool:            NewPool(isServerSide),
		lpool:           newListenPool(),
		outbound:        outbound,
		isessionManager: ism,
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

func (manager *Manager) tunnelCreate(cfg *TunnelConfig) (*Tunnel, error) {
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

// OpenTunnel open a tunnel
func (manager *Manager) OpenTunnel(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) error {
	// send open tunnel message to remote endpoint
	cfg := &TunnelConfig{
		LocalHost:  localHost,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Reverse:    reverse,
	}

	body, _ := json.Marshal(cfg.RemoteConfig())
	session, err := manager.isessionManager.New()
	if err != nil {
		logrus.Errorf("open isession failed: %s", err)
		return err
	}

	resp, err := session.SendAndWait(&isession.Request{
		Action: "/tunnel",
		Body:   body,
	})
	if err != nil {
		logrus.Errorf("send request to remote endpoint failed: %s", err)
		return err
	}

	// fmt.Println("resp: ", resp)
	if resp.Status != "success" {
		logrus.Errorf("open tunnel in the remote endpoint failed: %+v", resp)
		return errors.New("open tunnel in the remote endpoint failed")
	}

	tcBody := tunnelCreateBody{}
	if err = json.Unmarshal(resp.Body, &tcBody); err != nil {
		logrus.Errorf("json unmarshal body failed: %s", err)
		return errors.New("json unmarshal body error")
	}

	// success: open tunnel at local endpoint
	logrus.Debug("open tunnel in the remote endpoint success")

	cfg.ID = tcBody.ID
	t, err := manager.tunnelCreate(cfg)
	if err != nil {
		logrus.Errorf("open tunnel in the local side failed: %s", err)
		// TODO: close the tunnel in remote endpoint
		return errors.New("open tunnel in the local side failed")
	}

	logrus.Debugf("open tunnel %s in the local side success", t)

	return nil
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
