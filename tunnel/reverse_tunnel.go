package tunnel

import (
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/tunnel/channel"
	tcommon "github.com/ooclab/es/tunnel/common"
)

// ReverseTunnel define a forward tunnel struct
type ReverseTunnel struct {
	id       uint32
	config   *TunnelConfig
	cpool    *channel.Pool
	outbound chan *common.LinkOMSG
}

func newReverseTunnel(cfg *TunnelConfig, outbound chan *common.LinkOMSG) Tunneler {
	return &ReverseTunnel{
		id:       cfg.ID,
		config:   cfg,
		cpool:    channel.NewPool(),
		outbound: outbound,
	}
}

func (t *ReverseTunnel) String() string {
	cfg := t.config
	return fmt.Sprintf("%d L:%s:%d <- R:%s:%d", t.id, cfg.LocalHost, cfg.LocalPort, cfg.RemoteHost, cfg.RemotePort)
}

func (t *ReverseTunnel) ID() uint32 {
	return t.id
}

func (t *ReverseTunnel) Config() *TunnelConfig {
	return t.config
}

func (t *ReverseTunnel) HandleIn(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if c == nil {
		// (reverse tunnel) need to setup a connect to localhost:localport
		cfg := t.config
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
		c = t.cpool.NewByID(m.ChannelID, t.id, t.outbound, conn)
		go t.ServeChannel(c)
		logrus.Debugf("OPEN channel %s success", c)
	}

	// TODO: goroutine ?
	return c.HandleIn(m)
}

func (t *ReverseTunnel) NewChannelByConn(conn net.Conn) *channel.Channel {
	// this reserved for interface
	logrus.Errorf("reverse tunnel can not create channel use random ID!")
	return nil
}

func (t *ReverseTunnel) ServeChannel(c *channel.Channel) {
	if err := c.Serve(); err != nil {
		t.closeRemoteChannel(c.ChannelID)
	}
	if t.cpool.Exist(c.ChannelID) {
		t.cpool.Delete(c)
	}
}

func (t *ReverseTunnel) closeRemoteChannel(cid uint32) {
	m := &tcommon.TMSG{
		Type:      tcommon.MsgTypeChannelClose,
		TunnelID:  t.id,
		ChannelID: cid,
	}
	logrus.Debugf("Before: notice remote endpoint to close channel %d", cid)
	t.outbound <- &common.LinkOMSG{
		Type:    common.LinkMsgTypeTunnel,
		Payload: m.Bytes(),
	}
	logrus.Debugf("notice remote endpoint to close channel %d", cid)
}

func (t *ReverseTunnel) HandleChannelClose(m *tcommon.TMSG) error {
	c := t.cpool.Get(m.ChannelID)
	if c == nil {
		logrus.Warnf("can not find channel %d:%d", m.TunnelID, m.ChannelID)
		return errors.New("no such channel")
	}

	t.cpool.Delete(c)
	c.Close()
	return nil
}
