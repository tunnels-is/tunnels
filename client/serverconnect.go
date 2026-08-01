package client

import (
	"errors"

	"github.com/google/uuid"
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
	reconcileDefaultDevice(cr)
	return PublicConnect(cr)
}

func reconcileDefaultDevice(cr *ConnectionRequest) {
	meta := findTunnelMetaByTag(DefaultTunnelName)
	if meta == nil || meta.WireGuardPrivKey == "" {
		return
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
		return
	}
	INFO("server-connect: default device bound to another server, remaking: ", device.ID)
	if err := deleteDevice(form, device); err != nil {
		ERROR("server-connect: unable to delete mismatched device: ", err)
	}
}
