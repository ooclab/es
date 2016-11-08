package common

import "fmt"

// msg type
const (
	LinkMsgTypeInnerSession      = 1
	LinkMsgTypeTunnel            = 2
	LinkMsgTypeHeartbeatRequest  = 3
	LinkMsgTypeHeartbeatResponse = 4
)

const (
	DefaultLinkMSGVersion uint8  = 2
	DefaultLinkMSGFlag    uint16 = 0
)

// LinkOMSG is for link's outbound
type LinkOMSG struct {
	Type    uint8
	Payload []byte
}

var linkMsgHumanTypeMap = map[uint8]string{
	LinkMsgTypeInnerSession: "InnerSession",
	LinkMsgTypeTunnel:       "Tunnel",
}

func GetLinkMsgHumanType(_type uint8) string {
	if v, ok := linkMsgHumanTypeMap[_type]; ok {
		return v
	}

	return fmt.Sprintf("Unknown(%d)", _type)
}
