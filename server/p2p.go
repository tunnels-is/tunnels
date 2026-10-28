package main

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
	"github.com/tunnels-is/tunnels/types"
)

type p2pPolicyRequest struct {
	DeviceID string   `json:"DeviceID"`
	AllowAll bool     `json:"AllowAll"`
	AllowIPs []string `json:"AllowIPs"`
}

type p2pDiscoRequest struct {
	DeviceID string `json:"DeviceID"`
	DiscoKey string `json:"DiscoKey"`
}

func handleClientP2PPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}
	deviceID, err := uuid.Parse(r.URL.Query().Get("deviceID"))
	if err != nil {
		sendError(w, 400, "deviceID is required")
		return
	}
	self, err := findDeviceByID(deviceID)
	if err != nil || self == nil || self.UserID != user.ID {
		sendError(w, 404, "device not found")
		return
	}
	home, err := findServerByID(self.ServerID)
	if err != nil || home == nil {
		sendError(w, 404, "server not found")
		return
	}
	if code, msg := wgUserConnectStatus(user, home); code != 0 {
		sendError(w, code, msg)
		return
	}
	peers, err := punchPeersFor(self, home)
	if err != nil {
		sendError(w, 500, "Failed to list peers")
		return
	}
	sendObject(w, map[string]any{"Peers": peers})
}

func handleClientP2PDisco(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}
	form := new(p2pDiscoRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body")
		return
	}
	if _, err := punch.DecodeKey(form.DiscoKey); err != nil {
		sendError(w, 400, "invalid disco key")
		return
	}
	id, err := uuid.Parse(form.DeviceID)
	if err != nil {
		sendError(w, 400, "invalid device")
		return
	}
	dev, err := findDeviceByID(id)
	if err != nil || dev == nil || dev.UserID != user.ID {
		sendError(w, 404, "device not found")
		return
	}
	if err := setDeviceDiscoKey(id, form.DiscoKey); err != nil {
		sendError(w, 409, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleWGP2PPolicy(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}
	form := new(p2pPolicyRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body")
		return
	}
	id, err := uuid.Parse(form.DeviceID)
	if err != nil {
		sendError(w, 400, "invalid device")
		return
	}
	dev, err := findDeviceByID(id)
	if err != nil || dev == nil || dev.ServerID != server.ID {
		sendError(w, 404, "device not found")
		return
	}
	ips := make([]string, 0, len(form.AllowIPs))
	for _, raw := range form.AllowIPs {
		addr, err := netip.ParseAddr(raw)
		if err != nil || !addr.Is4() {
			continue
		}
		ips = append(ips, addr.String())
	}
	if err := setDevicePunchPolicy(id, form.AllowAll, ips); err != nil {
		sendError(w, 500, "Failed to store policy")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleWGP2PLocal(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}
	devices, err := listAllDevices()
	if err != nil {
		sendError(w, 500, "Failed to list devices")
		return
	}
	users := map[uuid.UUID]*User{}
	now := time.Now()
	out := make([]types.P2PPeer, 0)
	for _, d := range devices {
		if d == nil || d.ServerID != server.ID || d.DiscoKey == "" || d.WireGuardIP == "" || d.WireGuardKey == "" {
			continue
		}
		if !p2pOwnerAllowed(d, server, users, now) {
			continue
		}
		out = append(out, types.P2PPeer{
			DeviceID:     d.ID.String(),
			DiscoKey:     d.DiscoKey,
			WireGuardKey: d.WireGuardKey,
			WireGuardIP:  d.WireGuardIP,
		})
	}
	sendObject(w, map[string]any{"Peers": out})
}

func handleWGP2PLocate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("deviceID"))
	if err != nil {
		sendError(w, 400, "deviceID is required")
		return
	}
	dev, err := findDeviceByID(id)
	if err != nil || dev == nil {
		sendError(w, 404, "device not found")
		return
	}
	home, err := findServerByID(dev.ServerID)
	if err != nil || home == nil || !serversSharePunch(server, home) {
		sendError(w, 404, "device not found")
		return
	}
	if !p2pOwnerAllowed(dev, home, map[uuid.UUID]*User{}, time.Now()) {
		sendError(w, 404, "device not found")
		return
	}
	sendObject(w, types.P2PLocate{
		DeviceID:    dev.ID.String(),
		DiscoKey:    dev.DiscoKey,
		WireGuardIP: dev.WireGuardIP,
		HomeIP:      home.IP,
		HomePort:    home.WireGuardPort,
		Local:       home.ID == server.ID,
	})
}

func punchPeersFor(self *types.Device, home *types.Server) ([]types.P2PPeer, error) {
	devices, err := listAllDevices()
	if err != nil {
		return nil, err
	}
	servers, err := findAllServers(10000, 0)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*types.Server, len(servers))
	for _, s := range servers {
		if s != nil {
			byID[s.ID] = s
		}
	}
	selfIP, err := netip.ParseAddr(self.WireGuardIP)
	if err != nil {
		return nil, err
	}
	users := map[uuid.UUID]*User{}
	now := time.Now()
	out := make([]types.P2PPeer, 0)
	for _, d := range devices {
		if d == nil || d.ID == self.ID || d.DiscoKey == "" || d.WireGuardKey == "" || d.WireGuardIP == "" {
			continue
		}
		peerSrv := byID[d.ServerID]
		if peerSrv == nil || !serversSharePunch(home, peerSrv) {
			continue
		}
		if !p2pOwnerAllowed(d, peerSrv, users, now) {
			continue
		}
		peerIP, err := netip.ParseAddr(d.WireGuardIP)
		if err != nil || !peerIP.Is4() {
			continue
		}
		if !punch.PairAdmitted(!home.EnableFirewall, self.P2PAllowAll, self.P2PAllowIPs, selfIP, !peerSrv.EnableFirewall, d.P2PAllowAll, d.P2PAllowIPs, peerIP) {
			continue
		}
		out = append(out, types.P2PPeer{
			DeviceID:     d.ID.String(),
			DiscoKey:     d.DiscoKey,
			WireGuardKey: d.WireGuardKey,
			WireGuardIP:  d.WireGuardIP,
		})
	}
	return out, nil
}

func p2pOwnerAllowed(d *types.Device, server *types.Server, users map[uuid.UUID]*User, now time.Time) bool {
	if d == nil || users == nil {
		return false
	}
	owner, ok := users[d.UserID]
	if !ok {
		owner, _ = findUserByID(d.UserID)
		users[d.UserID] = owner
	}
	return userMayConnect(owner, server, now)
}

func serversSharePunch(a, b *types.Server) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID == b.ID {
		return true
	}
	return a.MeshGroupID != "" && a.MeshGroupID == b.MeshGroupID
}

func listAllDevices() ([]*types.Device, error) {
	var all []*types.Device
	for off := int64(0); off < 100000; off += 500 {
		batch, err := getDevices(500, off)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 500 {
			return all, nil
		}
	}
	return all, nil
}

func normalizeDiscoKey(s string) error {
	if s == "" {
		return nil
	}
	_, err := punch.DecodeKey(s)
	return err
}
