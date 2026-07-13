package wgserver

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/tunnels-is/tunnels/types"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// The server-to-server mesh: a second WireGuard interface (derived from the
// client iface, e.g. wg0 -> wg0mesh) whose peers are the sibling wg-servers in
// the same mesh group, as told by the controller (GET /wg/mesh). Cross-server
// client traffic is routed wg0 -> wgmesh -> sibling wgmesh -> sibling wg0, so
// the real client source IP is preserved end-to-end and the inter-server hop is
// encrypted + authenticated. Unlike wg0 there is no LazyBind and no inspector.

var (
	wgMeshDevice *device.Device

	meshMu    sync.Mutex
	meshPeers = map[string]installedMeshPeer{} // key: PublicKeyHex
)

// installedMeshPeer is the local record of a sibling currently configured on
// the mesh interface, so reconcileMesh can diff against the controller's list
// and know which routes to withdraw when a sibling leaves.
type installedMeshPeer struct {
	PublicKeyHex string
	Endpoint     string
	Subnets      []string
}

// meshIface derives the mesh interface name from the client interface.
func meshIface(cfg *Config) string {
	return cfg.WireGuardIface + "mesh"
}

// setupMesh brings up the mesh WireGuard interface (reusing the server static
// key) and installs its firewall rules. It is a no-op when no mesh port is
// configured. Peers are added later by reconcileMesh.
func setupMesh(cfg *Config, logLevel string) error {
	if cfg.WireGuardMeshPort == 0 {
		INFO("mesh: no mesh port configured, mesh disabled")
		return nil
	}

	iface := meshIface(cfg)

	priv, err := loadOrGenerateLocalPrivKey()
	if err != nil {
		return fmt.Errorf("mesh: load key: %w", err)
	}
	defer zeroBytes(priv)
	if len(priv) != 32 {
		return fmt.Errorf("mesh: invalid key length %d", len(priv))
	}

	tunDev, err := tun.CreateTUN(iface, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("mesh: CreateTUN %q: %w", iface, err)
	}

	wgLogger := device.NewLogger(wgDeviceLogLevel(logLevel), "["+iface+"] ")
	wgMeshDevice = device.NewDevice(tunDev, conn.NewDefaultBind(), wgLogger)

	privHex := make([]byte, hex.EncodedLen(len(priv)))
	hex.Encode(privHex, priv)
	conf := fmt.Appendf(nil, "private_key=%s\nlisten_port=%d\n\n", privHex, cfg.WireGuardMeshPort)
	zeroBytes(privHex)
	if err := ipcSetMesh(conf); err != nil {
		zeroBytes(conf)
		return fmt.Errorf("mesh: IpcSet: %w", err)
	}
	zeroBytes(conf)

	if err := wgMeshDevice.Up(); err != nil {
		return fmt.Errorf("mesh: device Up: %w", err)
	}

	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("mesh: link %q: %w", iface, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("mesh: LinkSetUp %q: %w", iface, err)
	}

	if err := addMeshRules(cfg); err != nil {
		return fmt.Errorf("mesh: firewall rules: %w", err)
	}

	INFO("mesh: ", iface, " up on port ", cfg.WireGuardMeshPort)
	return nil
}

func ipcSetMesh(conf []byte) error {
	if wgMeshDevice == nil {
		return fmt.Errorf("mesh device not initialized")
	}
	return wgMeshDevice.IpcSetOperation(bufio.NewReader(bytes.NewReader(conf)))
}

func ipcSetMeshStr(conf string) error {
	if wgMeshDevice == nil {
		return fmt.Errorf("mesh device not initialized")
	}
	return wgMeshDevice.IpcSetOperation(bufio.NewReader(strings.NewReader(conf)))
}

func addMeshPeer(pubKeyHex, endpoint string, allowedIPs []string) error {
	conf := fmt.Sprintf("public_key=%s\n", sanitizeIPC(pubKeyHex))
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", sanitizeIPC(aip))
	}
	conf += fmt.Sprintf("endpoint=%s\npersistent_keepalive_interval=25\n\n", sanitizeIPC(endpoint))
	return ipcSetMeshStr(conf)
}

func removeMeshPeer(pubKeyHex string) error {
	conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", sanitizeIPC(pubKeyHex))
	return ipcSetMeshStr(conf)
}

func addMeshRoute(cfg *Config, subnet string) error {
	link, err := netlink.LinkByName(meshIface(cfg))
	if err != nil {
		return err
	}
	_, dst, err := net.ParseCIDR(subnet)
	if err != nil {
		return err
	}
	return netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: dst})
}

func delMeshRoute(cfg *Config, subnet string) {
	link, err := netlink.LinkByName(meshIface(cfg))
	if err != nil {
		return
	}
	_, dst, err := net.ParseCIDR(subnet)
	if err != nil {
		return
	}
	_ = netlink.RouteDel(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: dst})
}

// fetchMesh asks the controller for this server's same-mesh-group siblings.
func fetchMesh(cfg *Config) (*types.WGMeshResponse, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.ControllerURL+"/wg/mesh", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned %d", resp.StatusCode)
	}
	var r types.WGMeshResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// reconcileMesh syncs the installed mesh peers with the controller's current
// same-mesh-group list: add new siblings, drop departed/changed ones, and keep
// their routes in step. A controller error is transient — the current mesh is
// left untouched. Called at startup and on a 2-minute timer.
func reconcileMesh() {
	cfg := activeConfig.Load()
	if cfg == nil || wgMeshDevice == nil {
		return
	}

	resp, err := fetchMesh(cfg)
	if err != nil {
		WARN("mesh: fetch failed (keeping current peers): ", err)
		return
	}

	desired := make(map[string]types.WGMeshPeer, len(resp.Peers))
	for _, p := range resp.Peers {
		if p.PublicKeyHex != "" {
			desired[p.PublicKeyHex] = p
		}
	}

	meshMu.Lock()
	defer meshMu.Unlock()

	// Drop peers that left the group or whose endpoint/subnets changed.
	for key, inst := range meshPeers {
		if want, ok := desired[key]; ok && sameMeshPeer(inst, want) {
			continue
		}
		_ = removeMeshPeer(key)
		for _, s := range inst.Subnets {
			delMeshRoute(cfg, s)
		}
		delete(meshPeers, key)
		INFO("mesh: removed peer ", short(key))
	}

	// Add new (or re-add changed) peers.
	for key, want := range desired {
		if _, ok := meshPeers[key]; ok {
			continue
		}
		if err := addMeshPeer(key, want.Endpoint, want.AllowedSubnets); err != nil {
			WARN("mesh: addMeshPeer failed: ", err)
			continue
		}
		for _, s := range want.AllowedSubnets {
			if err := addMeshRoute(cfg, s); err != nil {
				WARN("mesh: addMeshRoute ", s, " failed: ", err)
			}
		}
		meshPeers[key] = installedMeshPeer{PublicKeyHex: key, Endpoint: want.Endpoint, Subnets: want.AllowedSubnets}
		INFO("mesh: added peer ", short(key), " endpoint=", want.Endpoint)
	}
}

func sameMeshPeer(a installedMeshPeer, b types.WGMeshPeer) bool {
	if a.Endpoint != b.Endpoint || len(a.Subnets) != len(b.AllowedSubnets) {
		return false
	}
	for i := range a.Subnets {
		if a.Subnets[i] != b.AllowedSubnets[i] {
			return false
		}
	}
	return true
}

// cleanupMesh removes all mesh peers/routes and tears down the mesh device.
func cleanupMesh(cfg *Config) {
	meshMu.Lock()
	for key, inst := range meshPeers {
		_ = removeMeshPeer(key)
		for _, s := range inst.Subnets {
			delMeshRoute(cfg, s)
		}
		delete(meshPeers, key)
	}
	meshMu.Unlock()

	if wgMeshDevice != nil {
		wgMeshDevice.Close()
		wgMeshDevice = nil
	}
}

func short(hexKey string) string {
	if len(hexKey) <= 12 {
		return hexKey
	}
	return hexKey[:12] + "…"
}
