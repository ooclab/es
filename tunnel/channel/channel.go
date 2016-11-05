package channel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/util"

	tcommon "github.com/ooclab/es/tunnel/common"
)

type Channel struct {
	// !IMPORTANT! atomic.AddInt64 in arm / x86_32
	// https://plus.ooclab.com/note/article/1285
	recv uint64
	send uint64

	TunnelID  uint32
	ChannelID uint32
	Outbound  chan *common.LinkOMSG
	Conn      net.Conn

	closed bool

	lock *sync.Mutex
}

func (c *Channel) String() string {
	return fmt.Sprintf(`%d-%d: L(%s), R(%s)`, c.TunnelID, c.ChannelID, c.Conn.LocalAddr(), c.Conn.RemoteAddr())
}

func (c *Channel) Close() {
	if c.closed {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	c.closed = true
	closeConn(c.Conn)

	logrus.Debugf("CLOSE channel %s: recv = %d, send = %d", c, atomic.LoadUint64(&c.recv), atomic.LoadUint64(&c.send))
}

func (c *Channel) HandleIn(m *tcommon.TMSG) error {
	// TODO: 1. use write cached !
	// TODO: 2. use goroutine & channel to handle inbound message ?
	wLen, err := c.Conn.Write(m.Payload)
	// FIXME: make sure write all data, BUT it seems that golang do it already!
	if wLen != len(m.Payload) {
		logrus.Errorf("Channel c.Conn.Write error: wLen = %d != len(m.Payload) = %d", wLen, len(m.Payload))
	}
	if err != nil {
		logrus.Errorf("channel write failed: %s", err)
		return errors.New("write payload error")
	}

	atomic.AddUint64(&c.send, uint64(wLen))
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
			if c.closed || util.TCPisClosedConnError(err) {
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
		atomic.AddUint64(&c.recv, uint64(reqLen))
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
