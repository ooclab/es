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

// ForwardTunnel define a forward tunnel struct
type ForwardTunnel struct {
	id       uint32
	config   *TunnelConfig
	cpool    *channel.Pool
	outbound chan []byte
}

func newForwardTunnel(cfg *TunnelConfig, outbound chan []byte) Tunneler {
	return &ForwardTunnel{
		id:       cfg.ID,
		config:   cfg,
		cpool:    channel.NewPool(),
		outbound: outbound,
	}
}

func (t *ForwardTunnel) String() string {
	cfg := t.config
	return fmt.Sprintf("%d L:%s:%d -> R:%s:%d", t.id, cfg.LocalHost, cfg.LocalPort, cfg.RemoteHost, cfg.RemotePort)
}

func (t *ForwardTunnel) ID() uint32 {
	return t.id
}

func (t *ForwardTunnel) Config() *TunnelConfig {
	return t.config
}

func (t *ForwardTunnel) HandleIn(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if c == nil {
		logrus.Errorf("can not find channel %d:%d", m.TunnelID, m.ChannelID)
		return errors.New("no such channel")
	}

	// TODO: goroutine ?
	return c.HandleIn(m)
}

func (t *ForwardTunnel) NewChannelByConn(conn net.Conn) *channel.Channel {
	return t.cpool.New(t.id, t.outbound, conn)
}

func (t *ForwardTunnel) ServeChannel(c *channel.Channel) {
	if err := c.Serve(); err != nil {
		t.closeRemoteChannel(c.ChannelID)
	}
	if t.cpool.Exist(c.ChannelID) {
		t.cpool.Delete(c)
	}
}

func (t *ForwardTunnel) closeRemoteChannel(cid uint32) {
	logrus.Debugf("prepare notice remote endpoint to close channel %d", cid)
	m := &tcommon.TMSG{
		Type:      tcommon.MsgTypeChannelClose,
		TunnelID:  t.id,
		ChannelID: cid,
	}
	// FIXME! panic: send on closed channel
	t.outbound <- append([]byte{es.LinkMsgTypeTunnel}, m.Bytes()...)
	logrus.Debugf("notice remote endpoint to close channel %d done", cid)
}

func (t *ForwardTunnel) HandleChannelClose(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if c == nil {
		logrus.Warnf("can not find channel %d:%d", m.TunnelID, m.ChannelID)
		return errors.New("no such channel")
	}

	t.cpool.Delete(c)
	c.Close()
	return nil
}
