package tunnel

import (
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es"
	"github.com/ooclab/es/tunnel/channel"
	tcommon "github.com/ooclab/es/tunnel/common"
)

type TunnelConfig struct {
	ID         uint32
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Reverse    bool
}

func (c *TunnelConfig) RemoteConfig() *TunnelConfig {
	return &TunnelConfig{
		LocalHost:  c.RemoteHost,
		LocalPort:  c.RemotePort,
		RemoteHost: c.LocalHost,
		RemotePort: c.LocalPort,
		Reverse:    !c.Reverse,
	}
}

// Tunnel define a tunnel struct
type Tunnel struct {
	ID       uint32
	Config   *TunnelConfig
	cpool    *channel.Pool
	outbound chan []byte
}

func newTunnel(cfg *TunnelConfig, outbound chan []byte) *Tunnel {
	return &Tunnel{
		ID:       cfg.ID,
		Config:   cfg,
		cpool:    channel.NewPool(),
		outbound: outbound,
	}
}

func (t *Tunnel) String() string {
	cfg := t.Config
	if cfg.Reverse {
		return fmt.Sprintf("%d L:%s:%d <- R:%s:%d", t.ID, cfg.LocalHost, cfg.LocalPort, cfg.RemoteHost, cfg.RemotePort)
	}
	return fmt.Sprintf("%d L:%s:%d -> R:%s:%d", t.ID, cfg.LocalHost, cfg.LocalPort, cfg.RemoteHost, cfg.RemotePort)
}

func (t *Tunnel) HandleIn(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if t.Config.Reverse {
		if c == nil {
			// (reverse tunnel) need to setup a connect to localhost:localport
			cfg := t.Config
			addrS := fmt.Sprintf("%s:%d", cfg.LocalHost, cfg.LocalPort)
			addr, err := net.ResolveTCPAddr("tcp", addrS)
			if err != nil {
				logrus.Warnf("resolve %s failed: %s", addrS, err)
				// TODO: notice remote endpoint ?
				return err
			}

			conn, err := net.DialTCP("tcp", nil, addr)
			if err != nil {
				logrus.Errorf("dial %s failed: %s", addrS, err.Error())
				// TODO: try again ?
				return err
			}

			// IMPORTANT! create channel by ID!
			c = t.cpool.NewByID(m.ChannelID, t.ID, t.outbound, conn)
			go t.ServeChannel(c)
			logrus.Debugf("OPEN channel %s success", c)
		}
	} else {
		// forward tunnel
		if c == nil {
			logrus.Errorf("can not find channel %d:%d", m.TunnelID, m.ChannelID)
			return errors.New("no such channel")
		}
	}

	return c.HandleIn(m)
}

func (t *Tunnel) NewChannelByConn(conn net.Conn) *channel.Channel {
	if t.Config.Reverse {
		logrus.Errorf("reverse tunnel can not create channel use random ID!")
		return nil
	}
	return t.cpool.New(t.ID, t.outbound, conn)
}

func (t *Tunnel) ServeChannel(c *channel.Channel) {
	if err := c.Serve(); err != nil {
		t.closeRemoteChannel(c.ChannelID)
	}
	if t.cpool.Exist(c.ChannelID) {
		t.cpool.Delete(c)
	}
}

func (t *Tunnel) closeRemoteChannel(cid uint32) {
	logrus.Debugf("prepare notice remote endpoint to close channel %d", cid)
	m := &tcommon.TMSG{
		Type:      tcommon.MsgTypeChannelClose,
		TunnelID:  t.ID,
		ChannelID: cid,
	}
	// FIXME! panic: send on closed channel
	t.outbound <- append([]byte{es.LinkMsgTypeTunnel}, m.Bytes()...)
	logrus.Debugf("notice remote endpoint to close channel %d done", cid)
}

func (t *Tunnel) HandleChannelClose(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if c == nil {
		logrus.Warnf("can not find channel %d:%d", m.TunnelID, m.ChannelID)
		return errors.New("no such channel")
	}

	t.cpool.Delete(c)
	c.Close()
	return nil
}
