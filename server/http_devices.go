package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func handleAdminDeviceUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(updateDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	wgIPAllocMu.Lock()
	err = updateDevice(F.Device)
	wgIPAllocMu.Unlock()
	if err != nil {
		ERR(err)
		if errors.Is(err, errDeviceIPInUse) || errors.Is(err, errDeviceIPReserved) || errors.Is(err, errDeviceIPv6InUse) {
			senderr(w, 400, err.Error())
			return
		}
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminDeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(deleteDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = deleteDeviceByID(F.DID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleClientDeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getDeviceRequest)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	device, err := findDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		senderr(w, 404, "Device not found")
		return
	}

	if device.UserID != user.ID {
		senderr(w, 401, "You are not allowed to delete this device")
		return
	}

	if err := deleteDeviceByID(F.DeviceID); err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminDeviceList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listDevicesRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	devices, err := getDevices(int64(clampListLimit(F.Limit)), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, devices)
}

const (
	maxListLimit      = 10000
	defaultListLimit  = 500
	maxDevicesPerUser = 50
)

func clampListLimit(n int) int {
	if n <= 0 {
		return defaultListLimit
	}
	if n > maxListLimit {
		return maxListLimit
	}
	return n
}

func handleClientDeviceList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listDevicesRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	devices, err := getDevicesByUserID(user.ID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, devices)
}

func assignDeviceWireGuardIPs(d *types.Device) (err error, isIPv6 bool) {
	if d.ServerID == uuid.Nil {
		return nil, false
	}
	ip, assignErr := assignNextWireGuardIP(d.ServerID)
	if assignErr != nil {
		return assignErr, false
	}
	d.WireGuardIP = ip

	ipv6, assign6Err := assignNextWireGuardIPv6(d.ServerID)
	if assign6Err != nil {
		return assign6Err, true
	}
	d.WireGuardIPv6 = ipv6
	return nil, false
}

func deviceCreatePayload(device *types.Device, wgServer *types.Server) any {
	if wgServer != nil {
		return map[string]any{
			"Device":        device,
			"ServerPubKey":  wgServer.WireGuardPubKey,
			"ServerPort":    strconv.Itoa(wgServer.WireGuardPort),
			"ServerIP":      wgServer.IP,
			"ServerSubnet":  wgServer.WireGuardSubnet,
			"ServerSubnet6": wgServer.WireGuardSubnet6,
			"WANCIDR":       wanCIDRForServer(wgServer),
		}
	}
	return device
}

func handleClientDeviceCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(createDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Device == nil || F.Device.Tag == "" {
		senderr(w, 400, "Invalid device format")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		senderr(w, 403, "subscription expired")
		return
	}

	F.Device.UserID = user.ID
	F.Device.ID = uuid.New()
	F.Device.CreatedAt = time.Now()

	wgServer, srvErr := findServerByID(F.Device.ServerID)
	if srvErr != nil || wgServer == nil {
		senderr(w, 404, "Server not found")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, wgServer.Groups) {
		senderr(w, 401, "Unauthorized")
		return
	}

	if err := rejectServerWireGuardKey(F.Device.WireGuardKey); err != nil {
		senderr(w, 400, "invalid WireGuard key")
		return
	}

	wgIPAllocMu.Lock()
	defer wgIPAllocMu.Unlock()

	existing, listErr := getDevicesByUserID(user.ID)
	if listErr != nil {
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}
	if len(existing) >= maxDevicesPerUser {
		senderr(w, 400, "device limit reached for this account")
		return
	}

	if err, isIPv6 := assignDeviceWireGuardIPs(F.Device); err != nil {
		if isIPv6 {
			senderr(w, 400, "WireGuard IPv6 assignment failed", slog.Any("err", err))
			return
		}
		senderr(w, 400, "WireGuard IP assignment failed", slog.Any("err", err))
		return
	}

	err = createDevice(F.Device)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}

	sendObject(w, deviceCreatePayload(F.Device, wgServer))
}

func handleAdminDeviceCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(createDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Device == nil {
		senderr(w, 400, "No device given")
		return
	}

	if F.Device.Tag == "" {
		senderr(w, 400, "Missing device tag")
		return
	}

	if F.Device.UserID == uuid.Nil {
		senderr(w, 400, "Device UserID is required")
		return
	}

	F.Device.ID = uuid.New()
	F.Device.CreatedAt = time.Now()

	if err := rejectServerWireGuardKey(F.Device.WireGuardKey); err != nil {
		senderr(w, 400, "invalid WireGuard key")
		return
	}

	wgIPAllocMu.Lock()
	defer wgIPAllocMu.Unlock()

	var wgServer *types.Server
	if F.Device.ServerID != uuid.Nil {
		if err, isIPv6 := assignDeviceWireGuardIPs(F.Device); err != nil {
			if isIPv6 {
				senderr(w, 400, "WireGuard IPv6 assignment failed", slog.Any("err", err))
				return
			}
			senderr(w, 400, "WireGuard IP assignment failed", slog.Any("err", err))
			return
		}

		var srvErr error
		wgServer, srvErr = findServerByID(F.Device.ServerID)
		if srvErr != nil || wgServer == nil {
			senderr(w, 404, "Server not found")
			return
		}
	}

	err = createDevice(F.Device)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}

	sendObject(w, deviceCreatePayload(F.Device, wgServer))
}

func handleAdminDeviceGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	device, err := findDeviceByID(F.DeviceID)
	if err != nil {
		senderr(w, 400, "device not found", slog.Any("err", err))
		return
	}
	if device == nil {
		senderr(w, 400, "device not found")
		return
	}

	sendObject(w, device)
}

func handleClientDeviceGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getDeviceRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 400, "user not found")
		return
	}

	device, err := findDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		if err != nil {
			senderr(w, 400, "device  not found", slog.Any("err", err))
		} else {
			senderr(w, 400, "device not found")
		}
		return
	}

	if device.UserID != user.ID {
		senderr(w, 400, "unauthorized")
		return
	}

	sendObject(w, device)
}
