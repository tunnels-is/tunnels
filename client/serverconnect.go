package client

import (
	"errors"

	"github.com/google/uuid"
)

// ServerConnect connects to a specific server using the default tunnel.
//
// It differs from PublicConnect (which connects a tunnel to its own linked
// server): ServerConnect targets a user-chosen server and always uses the
// default tunnel, so that tunnel may migrate between servers over time.
// Because a device on the controller is bound to exactly one server, the
// default tunnel's device must be reconciled to the target first — if it is
// bound to a different server it is deleted, so PublicConnect re-creates it
// with an IP from the correct subnet. Without this, PublicConnect would reuse
// the stale device IP (from the previous server's subnet) against the new
// server.
func ServerConnect(cr *ConnectionRequest) (int, error) {
	if cr.ServerID == "" {
		ERROR("No server id found when connecting: ", cr)
		return 400, errors.New("no server id found when connecting")
	}
	cr.Tag = DefaultTunnelName
	reconcileDefaultDevice(cr)
	return PublicConnect(cr)
}

// reconcileDefaultDevice deletes the default tunnel's controller device when
// it is bound to a server other than cr.ServerID. Best-effort: on any lookup
// failure it leaves state as-is and lets PublicConnect proceed.
func reconcileDefaultDevice(cr *ConnectionRequest) {
	meta := findTunnelMetaByTag(DefaultTunnelName)
	if meta == nil || meta.WireGuardPrivKey == "" {
		return // no key yet -> no device exists, PublicConnect will create one
	}
	target, err := uuid.Parse(cr.ServerID)
	if err != nil {
		ERROR("server-connect: invalid server id: ", err)
		return
	}
	pubKey, err := deriveWGPubKey(meta.WireGuardPrivKey)
	if err != nil {
		return
	}

	// Reuse the auto-connect device helpers via an AutoConnectForm carrier;
	// they only read Server, DeviceToken and UserID.
	form := &AutoConnectForm{
		UserID:      cr.UserID,
		DeviceToken: cr.DeviceToken,
		Server:      cr.Server,
	}
	device, err := findDeviceByPubKey(form, pubKey)
	if err != nil {
		ERROR("server-connect: device lookup failed: ", err)
		return
	}
	if device == nil || device.ServerID == target {
		return // nothing on record, or already bound to the target server
	}
	INFO("server-connect: default device bound to another server, remaking: ", device.ID)
	if err := deleteDevice(form, device); err != nil {
		ERROR("server-connect: unable to delete mismatched device: ", err)
	}
}
