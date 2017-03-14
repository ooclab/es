package channel

import (
	tcommon "github.com/ooclab/es/tunnel/common"
)

type Channel interface {
	ID() uint32
	String() string
	Close()
	IsClosed() bool
	HandleIn(m *tcommon.TMSG) error
	Serve() error
}
