package ejson

import (
	"encoding/json"
	"io"

	"github.com/ooclab/es/emsg"
)

// Conn is a json proto connection
type Conn struct {
	conn *emsg.Conn
}

func NewConn(conn io.ReadWriteCloser) *Conn {
	return &Conn{
		conn: emsg.NewConn(conn),
	}
}

// Request send and recv payload
func (c *Conn) Request(payload interface{}) (map[string]interface{}, error) {
	if err := c.Send(payload); err != nil {
		return nil, err
	}
	return c.Recv()
}

// Recv recv a map[string]interface{} type payload
func (c *Conn) Recv() (map[string]interface{}, error) {
	// 等待回复
	msg, err := c.conn.Recv()
	if err != nil {
		return nil, err
	}

	// fmt.Printf("recv msg: %+v\n", msg)
	payload := map[string]interface{}{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// Send send a map[string]interface{} type payload
func (c *Conn) Send(payload interface{}) error {
	_payload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	m := &emsg.EMSG{
		Type:    emsg.MSG_TYPE_REQ,
		Payload: _payload,
	}
	return c.conn.Send(m)
}

// Close 关闭 connection
func (c *Conn) Close() {
	c.conn.Close()
}
