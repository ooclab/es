package isession

import (
	"github.com/ooclab/es/emsg"
)

const (
	MsgTypeRequest  uint8 = 1
	MsgTypeResponse uint8 = 2
)

// RequestHandler define request-response handler func
type RequestHandler interface {
	Handle(*emsg.EMSG) *emsg.EMSG
}
