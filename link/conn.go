package link

import (
	"encoding/binary"
	"io"

	"github.com/Sirupsen/logrus"
)

type Conn struct {
	conn io.ReadWriteCloser
}

func newConn(conn io.ReadWriteCloser) *Conn {
	c := &Conn{
		conn: conn,
	}
	return c
}

func (c *Conn) Recv() (*linkMSG, error) {
	hdr, err := c.mustRead(4)
	if err != nil {
		return nil, err
	}

	i := binary.LittleEndian.Uint32(hdr)

	msg := &linkMSG{
		Type:   uint8(i >> 24),
		Length: i & 0xffffff,
	}

	if msg.Length > linkMSGLengthMax {
		logrus.Debugf("read link msg: %s", msg)
		return nil, ErrMessageLengthTooLarge
	}

	if msg.Length > 0 {
		msg.Payload, err = c.mustRead(msg.Length)
		if err != nil {
			return nil, err
		}
	}

	return msg, nil
}

func (c *Conn) Send(m *linkMSG) error {
	// TODO: make sure write exactly data
	_, err := c.conn.Write(m.Bytes())
	return err
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) mustRead(dlen uint32) ([]byte, error) {
	data := make([]byte, dlen)
	for i := 0; i < int(dlen); {
		n, err := c.conn.Read(data[i:])
		if err != nil {
			return nil, err
		}
		i += n
	}
	return data, nil
}
