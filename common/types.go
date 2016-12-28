package common

import (
	"bytes"
	"encoding/binary"
	"io"
)

// message type
const (
	LinkMsgTypePingRequest  = 1
	LinkMsgTypePingResponse = 2
	LinkMsgTypeSession      = 10
	LinkMsgTypeTunnel       = 20
)

// LinkOMSG is a link's outbound message struct
type LinkOMSG struct {
	Type uint8
	Body []byte
}

// Reader get link message io.Reader
func (m *LinkOMSG) Reader() io.Reader {
	length := uint32(len(m.Body))
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, (uint32(m.Type)<<24)+length)

	if length > 0 {
		// binary.Write(buf, binary.LittleEndian, m.Payload)
		// FIXME!
		buf.Write(m.Body)
	}

	return bytes.NewReader(buf.Bytes())
}
