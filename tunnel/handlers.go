package tunnel

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/etp"
)

func (manager *Manager) HandleTunnelCreate(w etp.ResponseWriter, r *etp.Request) {
	cfg := &TunnelConfig{}
	if err := json.Unmarshal(r.Body, &cfg); err != nil {
		logrus.Errorf("tunnel create: unmarshal tunnel config failed: %s", err)
		etp.WriteJSON(w, etp.StatusExpectationFailed, map[string]string{"error": "load-tunnel-map-error"})
		return
	}

	logrus.Debug("handle got: ", cfg)

	manager.tunnelCreate(cfg)

	// fmt.Println("new tunnel ", tm)
	etp.WriteJSON(w, etp.StatusOK, map[string]interface{}{
		"tunnel_id": 123,
	})
}
