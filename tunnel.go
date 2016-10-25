package es

import "sync"

type TunnelConfig struct {
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Reverse    bool
}

// Tunnel define a tunnel struct
type Tunnel struct {
	ID     uint32
	config *TunnelConfig

	curChannelID     uint32
	channelIDMutex   *sync.Mutex
	channelPool      map[uint32]*Channel
	channelPoolMutex *sync.Mutex
}

func newTunnel(id uint32, config *TunnelConfig) *Tunnel {
	// TODO: setup tunnel

	return &Tunnel{
		ID:     id,
		config: config,

		curChannelID:     1,
		channelIDMutex:   &sync.Mutex{},
		channelPool:      map[uint32]*Channel{},
		channelPoolMutex: &sync.Mutex{},
	}
}
