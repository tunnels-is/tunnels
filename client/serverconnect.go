package client

import (
	"errors"
)

// ServerConnect brings up the default tunnel against the given server.
// PublicConnect persists ServerID onto that tunnel so AutoConnect can
// reuse it on the next launch.
func ServerConnect(cr *ConnectionRequest) (int, error) {
	if cr == nil {
		return 400, errors.New("connection request required")
	}
	cr.Tag = DefaultTunnelName
	return PublicConnect(cr)
}
