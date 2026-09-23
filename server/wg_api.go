package main

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

var wgIPAllocMu sync.Mutex

const (
	wgPeersDefaultLimit = 500
	wgPeersMaxLimit     = 5000
)

func handleWGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	limit, offset, errMsg := parseWGPeerPage(r)
	if errMsg != "" {
		sendError(w, 400, errMsg)
		return
	}

	devices, err := getDevices(int64(limit), int64(offset))
	if err != nil {
		sendError(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers:  make([]types.WGPeer, 0, len(devices)),
		Limit:  limit,
		Offset: offset,
	}

	userCache := make(map[uuid.UUID]*User)
	now := time.Now()
	for _, d := range devices {
		peer, ok := wgPeerFromDevice(d, server, userCache, now)
		if !ok {
			continue
		}
		resp.Peers = append(resp.Peers, peer)
	}

	if len(devices) < limit {
		resp.NextOffset = -1
	} else {
		resp.NextOffset = offset + limit
	}

	sendObject(w, resp)
}

func parseWGPeerPage(r *http.Request) (limit, offset int, errMsg string) {
	limit = wgPeersDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return 0, 0, "limit must be a positive integer"
		}
		if n > wgPeersMaxLimit {
			n = wgPeersMaxLimit
		}
		limit = n
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, "offset must be a non-negative integer"
		}
		offset = n
	}
	return limit, offset, ""
}

func wgPeerFromDevice(d *types.Device, server *types.Server, userCache map[uuid.UUID]*User, now time.Time) (types.WGPeer, bool) {
	if d.WireGuardKey == "" || d.ServerID != server.ID {
		return types.WGPeer{}, false
	}
	owner, ok := userCache[d.UserID]
	if !ok {
		owner, _ = findUserByID(d.UserID)
		userCache[d.UserID] = owner
	}
	if owner == nil || owner.Disabled {
		return types.WGPeer{}, false
	}
	if !owner.SubExpiration.IsZero() && now.After(owner.SubExpiration) {
		return types.WGPeer{}, false
	}
	if !hasSharedOrNoGroup(owner.Groups, server.Groups) {
		return types.WGPeer{}, false
	}
	hexKey, err := b64KeyToHex(d.WireGuardKey)
	if err != nil {
		return types.WGPeer{}, false
	}
	return types.WGPeer{
		PublicKeyHex:  hexKey,
		DeviceID:      d.ID.String(),
		WireGuardIP:   d.WireGuardIP,
		WireGuardIPv6: d.WireGuardIPv6,
	}, true
}

func handleWGPeer(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	pubKeyB64 := r.URL.Query().Get("pubkey")
	if pubKeyB64 == "" {
		sendError(w, 400, "pubkey query parameter is required")
		return
	}
	raw, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(raw) != 32 {
		sendError(w, 400, "pubkey must be a base64-encoded 32-byte key")
		return
	}

	dev, err := findDeviceByWGKey(pubKeyB64)
	if err != nil {
		sendError(w, 500, "Failed to look up device", slog.Any("err", err))
		return
	}
	if dev == nil {
		sendError(w, 404, "device not found")
		return
	}

	if dev.ServerID != server.ID {
		sendError(w, 401, "device not allowed on this server")
		return
	}

	user, err := findUserByID(dev.UserID)
	if err != nil {
		sendError(w, 500, "error looking up user")
		return
	}
	if code, msg := wgUserConnectStatus(user, server); code != 0 {
		sendError(w, code, msg)
		return
	}

	hexKey, err := b64KeyToHex(dev.WireGuardKey)
	if err != nil {
		sendError(w, 500, "Failed to encode device key", slog.Any("err", err))
		return
	}

	sendObject(w, types.WGPeer{
		PublicKeyHex:  hexKey,
		DeviceID:      dev.ID.String(),
		WireGuardIP:   dev.WireGuardIP,
		WireGuardIPv6: dev.WireGuardIPv6,
	})
}

func handleWGConfig(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	serverIDStr := r.URL.Query().Get("serverID")
	if serverIDStr == "" {
		sendError(w, 400, "serverID query parameter is required")
		return
	}
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		sendError(w, 400, "Invalid serverID")
		return
	}

	pubKey := r.URL.Query().Get("pubKey")
	if pubKey == "" {
		sendError(w, 400, "No pubkey given")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized - no user in context")
		return
	}

	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		sendError(w, 403, "subscription expired")
		return
	}

	server, err := findServerByID(serverID)
	if err != nil || server == nil {
		sendError(w, 404, "Server not found")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		sendError(w, 401, "Unauthorized - no group access")
		return
	}

	d, err := findDeviceByWGKey(pubKey)
	if err != nil {
		sendError(w, 500, "Database error looking up device")
		return
	}

	// Occupancy oracle: someone else's key looks the same as an unknown key.
	deviceIP, deviceIPv6 := ownedDeviceIPs(d, user.ID, serverID)

	sendObject(w, map[string]any{
		"WireGuardPubKey":  server.WireGuardPubKey,
		"WireGuardPort":    strconv.Itoa(server.WireGuardPort),
		"ServerIP":         server.IP,
		"WireGuardIP":      deviceIP,
		"WireGuardIPv6":    deviceIPv6,
		"WireGuardSubnet":  server.WireGuardSubnet,
		"WireGuardSubnet6": server.WireGuardSubnet6,
		"WANCIDR":          wanCIDRForServer(server),
		"EnableFirewall":   server.EnableFirewall,
	})
}

func wgUserConnectStatus(user *User, server *types.Server) (int, string) {
	if user == nil {
		return 401, "user/device not allowed to connect"
	}
	if user.Disabled {
		return 403, "user account is disabled"
	}
	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		return 403, "user subscription has expired"
	}
	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		return 401, "user/device not allowed to connect"
	}
	return 0, ""
}

func ownedDeviceIPs(d *types.Device, userID, serverID uuid.UUID) (ip, ipv6 string) {
	if d == nil || d.UserID != userID || d.ServerID != serverID {
		return "", ""
	}
	return d.WireGuardIP, d.WireGuardIPv6
}

func rejectServerWireGuardKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("WireGuard key required")
	}
	servers, err := findAllServers(10000, 0)
	if err != nil {
		return err
	}
	for _, s := range servers {
		if s == nil || s.WireGuardPubKey == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(s.WireGuardPubKey), []byte(key)) == 1 {
			return fmt.Errorf("WireGuard key is reserved")
		}
	}
	return nil
}

func b64KeyToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return fmt.Sprintf("%x", b), nil
}

func serverFromWGKey(r *http.Request) (*types.Server, bool) {
	key := r.Header.Get("X-WG-KEY")
	if key == "" {
		return nil, false
	}
	s, err := findServerByAPIKey(key)
	if err != nil || s == nil {
		return nil, false
	}
	return s, true
}

func validateCIDR(s string) error {
	if s == "" {
		return nil
	}
	_, _, err := net.ParseCIDR(s)
	return err
}

func handleWGServerConfigFetch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	pubKeyB64 := r.Header.Get("X-WG-PubKey")
	if err := pinWireGuardPubKey(server, pubKeyB64); err != nil {
		sendError(w, http.StatusConflict, err.Error())
		return
	}

	resp := &types.WGServerConfigResponse{
		ServerID:          server.ID.String(),
		ServerIP:          server.IP,
		WireGuardPort:     server.WireGuardPort,
		WireGuardMeshPort: meshPortForServer(server),
		WireGuardSubnet:   server.WireGuardSubnet,
		WireGuardSubnet6:  server.WireGuardSubnet6,
		WireGuardIface:    server.WireGuardIface,
		InternetIface:     server.InternetIface,
		EnableFirewall:    server.EnableFirewall,
	}

	sendObject(w, resp)
}

// pinWireGuardPubKey records the node's pubkey on first bind. A different
// key is rejected until an admin rotates the server API key (which clears
// WireGuardPubKey). Same key on restart is a no-op.
func pinWireGuardPubKey(server *types.Server, pubKeyB64 string) error {
	if pubKeyB64 == "" {
		return nil
	}
	if server.WireGuardPubKey == "" {
		if err := setServerWireGuardPubKey(server.ID, pubKeyB64); err != nil {
			return err
		}
		server.WireGuardPubKey = pubKeyB64
		return nil
	}
	if pubKeyB64 == server.WireGuardPubKey {
		return nil
	}
	return fmt.Errorf("WireGuard public key is pinned; rotate the server API key to replace it")
}

func meshPortForServer(s *types.Server) int {
	if s.WireGuardMeshPort != 0 {
		return s.WireGuardMeshPort
	}
	return s.WireGuardPort + 1
}

func handleWGMesh(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	server := getServerFromContext(r.Context())
	if server == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	resp := types.WGMeshResponse{Peers: make([]types.WGMeshPeer, 0)}
	if server.MeshGroupID == "" {
		sendObject(w, resp)
		return
	}

	siblings, err := findServersByMeshGroup(server.MeshGroupID)
	if err != nil {
		sendError(w, 500, "Failed to fetch mesh peers", slog.Any("err", err))
		return
	}

	for _, s := range siblings {
		if s.ID == server.ID {
			continue
		}
		if s.WireGuardPubKey == "" || s.IP == "" || s.WireGuardSubnet == "" {
			continue
		}
		hexKey, err := b64KeyToHex(s.WireGuardPubKey)
		if err != nil {
			continue
		}
		subnets := []string{s.WireGuardSubnet}
		if s.WireGuardSubnet6 != "" {
			subnets = append(subnets, s.WireGuardSubnet6)
		}
		resp.Peers = append(resp.Peers, types.WGMeshPeer{
			PublicKeyHex:   hexKey,
			Endpoint:       net.JoinHostPort(s.IP, strconv.Itoa(meshPortForServer(s))),
			AllowedSubnets: subnets,
		})
	}

	sendObject(w, resp)
}
