package es

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/ooclab/es/ecrypt"
)

const (
	maxMessageLength = 1024 * 1024 * 16
)

// common error define
var (
	ErrBufferIsShort  = errors.New("buffer is short")
	ErrMaxLengthLimit = errors.New("max length limit")
)

// Conn is a interface a Conn
type Conn interface {
	Recv() (message []byte, err error)
	Send(message []byte) error
	Close() error
	SetMaxLength(uint32)
}

// BaseConn is the basic connection type
type BaseConn struct {
	conn   io.ReadWriteCloser
	maxLen uint32 // the max length of payload
}

// NewBaseConn create a base Conn object
func NewBaseConn(conn io.ReadWriteCloser) Conn {
	return &BaseConn{
		conn:   conn,
		maxLen: maxMessageLength, // 16M
	}
}

// SetMaxLength set the max length of payload
func (c *BaseConn) SetMaxLength(maxLen uint32) {
	c.maxLen = maxLen
}

// Recv read a message from this Conn
func (c *BaseConn) Recv() (message []byte, err error) {
	head, err := c.mustRecv(4)
	if err != nil {
		return
	}
	dlen := binary.BigEndian.Uint32(head)
	if dlen > c.maxLen {
		return nil, ErrMaxLengthLimit
	}
	return c.mustRecv(dlen)
}

// Send send a message to this Conn
func (c *BaseConn) Send(message []byte) error {
	dlen := uint32(len(message))
	if dlen > c.maxLen {
		return ErrMaxLengthLimit
	}

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, dlen)
	if err != nil {
		return err
	}
	err = binary.Write(buf, binary.BigEndian, message)
	if err != nil {
		return err
	}

	// TODO: make sure write exactly data
	_, err = c.conn.Write(buf.Bytes())
	return err
}

// Close close a Conn
func (c *BaseConn) Close() error {
	return c.conn.Close()
}

func (c *BaseConn) mustRecv(dlen uint32) ([]byte, error) {
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

// SafeConn ecrypt Conn
type SafeConn struct {
	BaseConn
	cipher *ecrypt.Cipher
}

// NewSafeConn create a safe Conn
func NewSafeConn(conn io.ReadWriteCloser, cipher *ecrypt.Cipher) Conn {
	c := &SafeConn{
		cipher: cipher,
	}
	c.conn = conn
	c.maxLen = maxMessageLength
	return c
}

// Recv read a message from this Conn
func (c *SafeConn) Recv() (message []byte, err error) {
	head, err := c.mustRecv(4)
	if err != nil {
		return
	}
	c.cipher.Decrypt(head[0:4], head[0:4])
	dlen := binary.BigEndian.Uint32(head)
	if dlen > c.maxLen {
		return nil, ErrMaxLengthLimit
	}
	message, err = c.mustRecv(dlen)
	if err == nil {
		c.cipher.Decrypt(message, message)
	}
	return
}

// Send send a message to this Conn
func (c *SafeConn) Send(message []byte) error {
	dlen := uint32(len(message))
	if dlen > c.maxLen {
		return ErrMaxLengthLimit
	}

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, dlen)
	if err != nil {
		return err
	}
	err = binary.Write(buf, binary.BigEndian, message)
	if err != nil {
		return err
	}

	// TODO: make sure write exactly data
	b := buf.Bytes()
	c.cipher.Encrypt(b, b)
	_, err = c.conn.Write(b)
	return err
}
