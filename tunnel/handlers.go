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

	t, err := manager.tunnelCreate(cfg)
	if err != nil {
		logrus.Errorf("create tunnel failed: %s", err)
		resp.Status = "create-tunnel-failed"
		return
	}

	body, _ := json.Marshal(tunnelCreateBody{ID: t.ID()})
	resp = &isession.Response{
		Status: "success",
		Body:   body,
	}
	return
}
