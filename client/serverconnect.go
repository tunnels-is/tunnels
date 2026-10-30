package client

import (
	"errors"
)

// ServerConnect ...
func ServerConnect(cr *ConnectionRequest) (int, error) {
	if cr == nil {
		return 400, errors.New("connection request required")
	}
	cr.Tag = DefaultTunnelName
	return PublicConnect(cr)
}
