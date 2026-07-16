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

	// device.NewDevice immediately starts worker goroutines and holds the TUN fd.
	// Every error path below must tear it down, otherwise a non-fatal mesh setup
	// failure (missing ip6tables, LinkSetUp EPERM, ...) leaks the device and its
	// goroutines for the whole process lifetime.
	success := false
	defer func() {
		if !success {
			wgMeshDevice.Close()
			wgMeshDevice = nil
		}
	}()

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
	success = true
	return nil
}

// validMeshSubnet accepts only a specific (non-default) CIDR. A mesh sibling is
// meant to advertise concrete subnets; a /0 (0.0.0.0/0 or ::/0) from the
// controller would install a default route/allowed-ip and silently hijack all
// of this server's egress to the sibling. This mirrors the validation the
// client-peer path already does in sync.go's reconcilePeer.
func validMeshSubnet(subnet string) bool {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}
	ones, _ := ipnet.Mask.Size()
	return ones != 0
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
	if cfg == nil {
		return
	}

	// fetchMesh does network I/O, so it runs outside meshMu. All wgMeshDevice
	// access happens under the lock below, where cleanupMesh's close+nil is also
	// serialized — otherwise a shutdown mid-reconcile is a data race (and an
	// IpcSet on an already-closed device).
	resp, err := fetchMesh(cfg)
	if err != nil {
		WARN("mesh: fetch failed (keeping current peers): ", err)
		return
	}

	// Filter each peer's subnets to the valid (specific, non-default) set HERE,
	// once, so the drop-loop comparison and the add-loop install both operate on
	// the same list. Filtering only at add time would make sameMeshPeer compare a
	// stored filtered list against the raw controller list and tear the peer down
	// and re-add it on every reconcile. A peer left with no valid subnets is
	// simply not desired (dropped if currently installed).
	desired := make(map[string]types.WGMeshPeer, len(resp.Peers))
	for _, p := range resp.Peers {
		if p.PublicKeyHex == "" {
			continue
		}
		subnets := make([]string, 0, len(p.AllowedSubnets))
		for _, s := range p.AllowedSubnets {
			if !validMeshSubnet(s) {
				WARN("mesh: rejecting invalid/overbroad subnet ", s, " from peer ", short(p.PublicKeyHex))
				continue
			}
			subnets = append(subnets, s)
		}
		if len(subnets) == 0 {
			WARN("mesh: peer ", short(p.PublicKeyHex), " has no valid subnets, skipping")
			continue
		}
		p.AllowedSubnets = subnets
		desired[p.PublicKeyHex] = p
	}

	meshMu.Lock()
	defer meshMu.Unlock()

	// cleanupMesh may have torn the device down while we were fetching.
	if wgMeshDevice == nil {
		return
	}

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

	// Add new (or re-add changed) peers. desired already holds only valid subnets.
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
	// Compare as a multiset — the controller's subnet ordering isn't guaranteed
	// stable, and treating a reordering as a change would needlessly tear down
	// and re-add the peer (dropping routes / blipping the mesh) every reconcile.
	// Counts matter: comparing as a plain set would treat [X,Y] and [X,X] as
	// equal (both lengths 2, both members present) and miss that a subnet was
	// replaced by a duplicate.
	counts := make(map[string]int, len(a.Subnets))
	for _, s := range a.Subnets {
		counts[s]++
	}
	for _, s := range b.AllowedSubnets {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

// cleanupMesh removes all mesh peers/routes and tears down the mesh device.
func cleanupMesh(cfg *Config) {
	meshMu.Lock()
	defer meshMu.Unlock()
	for key, inst := range meshPeers {
		_ = removeMeshPeer(key)
		for _, s := range inst.Subnets {
			delMeshRoute(cfg, s)
		}
		delete(meshPeers, key)
	}

	// Close + nil the device under the lock so a concurrent reconcileMesh
	// (holding meshMu) can't use it mid-teardown.
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
