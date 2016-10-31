package tunnel

import (
	"encoding/binary"
	"fmt"
)

const (
	msgLength int = 8
)

// msg type
const (
	msgTypeTunnelOpen   uint8 = 1
	msgTypeTunnelStream uint8 = 2
	msgTypeTunnelClose  uint8 = 3
)

var msgHumanTypeMap = map[uint8]string{
	msgTypeTunnelOpen:   "TunnelOpen",
	msgTypeTunnelStream: "TunnelStream",
	msgTypeTunnelClose:  "TunnelClose",
}

type tunnelMSG struct {
	Version uint8
	Type    uint8
	Flag    uint16
	ID      uint32
	Payload []byte
}

func (f *tunnelMSG) String() string {
	return fmt.Sprintf("msg V:%d T:%s F:%d ID:%d",
		f.Version, getMSGHumanType(f.Type), f.Flag, f.ID)
}

func getMSGHumanType(_type uint8) string {
	if v, ok := msgHumanTypeMap[_type]; ok {
		return v
	}

	return fmt.Sprintf("Unknown(%d)", _type)
}

func loadMSG(data []byte) *tunnelMSG {
	return &tunnelMSG{
		Version: data[0],
		Type:    data[1],
		Flag:    binary.LittleEndian.Uint16(data[2:4]),
		ID:      binary.LittleEndian.Uint32(data[4:8]),
		Payload: data[8:],
	}
}
