package link

import "errors"

var (
	ErrMessageLengthTooLarge = errors.New("the length of message is too large")
	ErrMessageTypeUnknown    = errors.New("unknown message type")
)
