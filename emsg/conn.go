package emsg

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

type Conn struct {
	conn io.ReadWriteCloser
}

func NewConn(conn io.ReadWriteCloser) *Conn {
	return &Conn{
		conn: conn,
	}
}

func (c *Conn) Recv() (*EMSG, error) {
	head, err := c.mustRecv(4)
	if err != nil {
		return nil, err
	}

	var dlen uint32
	dlen = binary.LittleEndian.Uint32(head)

	data, err := c.mustRecv(dlen)
	if err != nil {
		return nil, err
	}

	return LoadEMSG(data)
}

func (c *Conn) Send(m *EMSG) error {
	b := m.Bytes()
	buf := new(bytes.Buffer)

	dlen := uint32(len(b))
	err := binary.Write(buf, binary.LittleEndian, dlen)
	if err != nil {
		return err
	}
	err = binary.Write(buf, binary.LittleEndian, b)
	if err != nil {
		return err
	}

	// TODO: make sure write exactly data
	_, err = c.conn.Write(buf.Bytes())
	return err
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) CloseRead() {
	if conn, ok := c.conn.(*net.TCPConn); ok {
		conn.CloseRead()
	}
}

func (c *Conn) CloseWrite() {
	if conn, ok := c.conn.(*net.TCPConn); ok {
		conn.CloseWrite()
	}
}

func (c *Conn) mustRecv(dlen uint32) ([]byte, error) {
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
