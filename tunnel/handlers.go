package tunnel

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/isession"
)

func (manager *Manager) HandleTunnelCreate(r *isession.Request) (resp *isession.Response, err error) {
	cfg := &TunnelConfig{}
	if err = json.Unmarshal(r.Body, &cfg); err != nil {
		logrus.Errorf("tunnel create: unmarshal tunnel config failed: %s", err)
		resp.Status = "load-tunnel-map-error"
		return
	}

	logrus.Debug("handle got: ", cfg)

	manager.tunnelCreate(cfg)

	// fmt.Println("new tunnel ", tm)
	body, _ := json.Marshal(map[string]interface{}{
		"tunnel_id": 123,
	})
	resp = &isession.Response{
		Status: "success",
		Body:   body,
	}
	return
}
