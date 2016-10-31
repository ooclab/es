package link

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ooclab/es/common"
)

const (
	linkMSGHeaderLength = 8
	linkMSGLengthMax    = uint32(16777216 - 8) // 16M
)

type linkMSG struct {
	Version uint8
	Type    uint8
	Flag    uint16 // use 2**16 = 65536
	Length  uint32
	Payload []byte
}

func (m *linkMSG) String() string {
	return fmt.Sprintf("linkMSG V:%d T:%s F:%d L:%d",
		m.Version, common.GetLinkMsgHumanType(m.Type), m.Flag, m.Length)
}

func (m *linkMSG) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, m.Version)
	binary.Write(buf, binary.LittleEndian, m.Type)
	binary.Write(buf, binary.LittleEndian, m.Flag)

	length := uint32(len(m.Payload))
	binary.Write(buf, binary.LittleEndian, length)

	if length > 0 {
		binary.Write(buf, binary.LittleEndian, m.Payload)
	}

	return buf.Bytes()
}
