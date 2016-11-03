package channel

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"

	tcommon "github.com/ooclab/es/tunnel/common"
)

type Channel struct {
	TunnelID  uint32
	ChannelID uint32
	Outbound  chan *common.LinkOMSG
	Conn      net.Conn

	recv uint64
	send uint64

	closed bool
}

func (c *Channel) String() string {
	return fmt.Sprintf(`%d-%d: L(%s), R(%s)`, c.TunnelID, c.ChannelID, c.Conn.LocalAddr(), c.Conn.RemoteAddr())
}

func (c *Channel) Close() {
	closeConn(c.Conn)
	c.closed = true
	logrus.Debugf("CLOSE channel %s: recv = %d, send = %d", c, c.recv, c.send)
}

func (c *Channel) HandleIn(m *tcommon.TMSG) error {
	// TODO: use write cached !
	wLen, err := c.Conn.Write(m.Payload)
	if err != nil {
		logrus.Errorf("channel write failed: %s", err)
		return errors.New("write payload error")
	}

	c.send += uint64(wLen)
	return nil
}

func (c *Channel) Serve() error {
	// logrus.Debugf("start serve channel %s", c)

	// FIXME!
	defer func() {
		if !c.closed {
			c.Close()
		}
	}()

	// link.Outbound <- channel.conn.Read
	for {
		buf := make([]byte, 1024*64) // TODO: custom
		reqLen, err := c.Conn.Read(buf)
		if err != nil {
			if c.closed {
				logrus.Debugf("channel %s is closed normally, quit read", c)
				return nil
			}
			if err != io.EOF {
				logrus.Warnf("channel %s recv failed: %s", c, err)
			}

			return err
		}

		m := &tcommon.TMSG{
			Type:      tcommon.MsgTypeChannelForward,
			TunnelID:  c.TunnelID,
			ChannelID: c.ChannelID,
			Payload:   buf[:reqLen],
		}
		c.Outbound <- &common.LinkOMSG{
			Type:    common.LinkMsgTypeTunnel,
			Payload: m.Bytes(),
		}
		c.recv += uint64(reqLen)
	}
}

func closeConn(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("closeConn recovered: ", r)
		}
	}()
	conn.Close()
}
