package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/common"
	"github.com/ooclab/es/isession"
)

type Manager struct {
	pool            *Pool
	outbound        chan *common.LinkOMSG
	isessionManager *isession.Manager
}

func NewManager(outbound chan *common.LinkOMSG, ism *isession.Manager) *Manager {
	return &Manager{
		pool:            NewPool(),
		outbound:        outbound,
		isessionManager: ism,
	}
}

func (manager *Manager) HandleIn(payload []byte) error {
	if len(payload) < msgLength {
		return errors.New("tunnel frame is too short")
	}
	msg := loadMSG(payload)
	fmt.Println("load tunnel msg =", msg)
	return nil
}

func (manager *Manager) TunnelCreate(config *TunnelConfig) error {
	fmt.Println("-- create tunnel: ", config)
	return nil
}

// func (manager *Manager) OpenTunnel(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) error {
// 	fmt.Println("-- manager.OpenTunnel: ", localHost, localPort, remoteHost, remotePort, reverse)
// 	s, err := manager.isessionManager.New()
// 	if err != nil {
// 		logrus.Errorf("open isession failed: %s", err)
// 		return err
// 	}
//
// 	resp, err := s.Post("/tunnel", []byte("Hello, world!"))
// 	if err != nil {
// 		logrus.Error("request failed:", resp, err)
// 		return err
// 	}
// 	fmt.Println("resp, err = ", resp, err)
// 	fmt.Println("resp.Body = ", string(resp.Body))
// 	return nil
// }

func (manager *Manager) tunnelCreate(cfg *TunnelConfig) error {
	fmt.Println("tunnelCreate: ", cfg)
	return nil
}

// OpenTunnel open a tunnel
func (manager *Manager) OpenTunnel(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) error {
	// send open tunnel message to remote endpoint
	cfg := &TunnelConfig{
		LocalHost:  localHost,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Reverse:    reverse,
	}
	body, _ := json.Marshal(cfg)
	session, err := manager.isessionManager.New()
	if err != nil {
		logrus.Errorf("open isession failed: %s", err)
		return err
	}

	resp, err := session.Post("/tunnel", body)
	if err != nil {
		logrus.Errorf("send request to remote endpoint failed:", err)
		return err
	}

	respBody := responseTunnelCreate{}
	if err := json.Unmarshal(resp.Body, &respBody); err != nil {
		logrus.Errorf("unmarshal tunnel create response failed: %s", err)
		return err
	}

	fmt.Println("respBody: ", respBody)
	if respBody.Error != "" {
		logrus.Errorf("open tunnel in the remote endpoint failed: %+v", respBody)
		return errors.New("open tunnel in the remote endpoint failed")
	}

	// success: open tunnel at local endpoint
	logrus.Debug("open tunnel in the remote endpoint success")

	if err := manager.tunnelCreate(cfg); err != nil {
		logrus.Errorf("open tunnel in the local side failed: %s", err)
		// TODO: close the tunnel in remote endpoint
		return errors.New("open tunnel in the local side failed")
	}

	logrus.Debug("open tunnel in the local side success")

	return nil
}
