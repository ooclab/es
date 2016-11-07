package link

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ooclab/es/common"
)

const (
	linkMSGHeaderLength = 4
	linkMSGLengthMax    = 4194304 - 4 // 4M
)

// msg header is a uint32
// type(8bit) + null(2bit) + length(22bit)

type linkMSG struct {
	Type    uint8
	Length  uint32
	Payload []byte
}

func (m *linkMSG) String() string {
	return fmt.Sprintf("linkMSG T:%s L:%d",
		common.GetLinkMsgHumanType(m.Type), m.Length)
}

func (m *linkMSG) Bytes() []byte {
	length := uint32(len(m.Payload))
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, (uint32(m.Type)<<24)+length)

	if length > 0 {
		// binary.Write(buf, binary.LittleEndian, m.Payload)
		// FIXME!
		buf.Write(m.Payload)
	}

	return buf.Bytes()
}
