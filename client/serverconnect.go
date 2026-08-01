package client

import (
	"errors"
)

func ServerConnect(cr *ConnectionRequest) (int, error) {
	if cr.ServerID == "" {
		ERROR("No server id found when connecting: ", cr)
		return 400, errors.New("no server id found when connecting")
	}

	if err := authorizeControlServer(cr.Server); err != nil {
		return 403, err
	}
	cr.Tag = DefaultTunnelName
	// Device identity is resolved by ServerID inside PublicConnect (local devices/).
	return PublicConnect(cr)
}
