package link

import (
	"encoding/json"
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/session"
	"github.com/ooclab/es/tunnel"
)

func defaultOpenTunnel(sessionManager *session.Manager, tunnelManager *tunnel.Manager) OpenTunnelFunc {
	return func(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) error {
		// send open tunnel message to remote endpoint
		cfg := &tunnel.TunnelConfig{
			LocalHost:  localHost,
			LocalPort:  localPort,
			RemoteHost: remoteHost,
			RemotePort: remotePort,
			Reverse:    reverse,
		}

		body, _ := json.Marshal(cfg.RemoteConfig())
		s, err := sessionManager.New()
		if err != nil {
			logrus.Errorf("open session failed: %s", err)
			return err
		}

		resp, err := s.SendAndWait(&session.Request{
			Action: "/tunnel",
			Body:   body,
		})
		if err != nil {
			logrus.Errorf("send request to remote endpoint failed: %s", err)
			return err
		}

		// fmt.Println("resp: ", resp)
		if resp.Status != "success" {
			logrus.Errorf("open tunnel in the remote endpoint failed: %+v", resp)
			return errors.New("open tunnel in the remote endpoint failed")
		}

		tcBody := tunnelCreateBody{}
		if err = json.Unmarshal(resp.Body, &tcBody); err != nil {
			logrus.Errorf("json unmarshal body failed: %s", err)
			return errors.New("json unmarshal body error")
		}

		// success: open tunnel at local endpoint
		logrus.Debug("open tunnel in the remote endpoint success")

		cfg.ID = tcBody.ID
		t, err := tunnelManager.TunnelCreate(cfg)
		if err != nil {
			logrus.Errorf("open tunnel in the local side failed: %s", err)
			// TODO: close the tunnel in remote endpoint
			return errors.New("open tunnel in the local side failed")
		}

		logrus.Debugf("open tunnel %s in the local side success", t)

		return nil
	}
}
