package tunnel

import (
	"net"

	"github.com/ooclab/es/tunnel/channel"
	tcommon "github.com/ooclab/es/tunnel/common"
)

type TunnelConfig struct {
	ID         uint32
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Reverse    bool
}

func (c *TunnelConfig) RemoteConfig() *TunnelConfig {
	return &TunnelConfig{
		LocalHost:  c.RemoteHost,
		LocalPort:  c.RemotePort,
		RemoteHost: c.LocalHost,
		RemotePort: c.LocalPort,
		Reverse:    !c.Reverse,
	}
}

type Tunneler interface {
	ID() uint32
	Config() *TunnelConfig
	HandleIn(*tcommon.TMSG) error
	NewChannelByConn(net.Conn) *channel.Channel
	ServeChannel(*channel.Channel)
	HandleChannelClose(*tcommon.TMSG) error
}
