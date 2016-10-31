package tunnel

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
}

func newTunnel(id uint32, config *TunnelConfig) *Tunnel {
	// TODO: setup tunnel

	return &Tunnel{
		ID:     id,
		config: config,
	}
}
