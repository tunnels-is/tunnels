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

var (
	wgMeshDevice *device.Device

	meshMu    sync.Mutex
	meshPeers = map[string]installedMeshPeer{}
)

type installedMeshPeer struct {
	PublicKeyHex string
	Endpoint     string
	Subnets      []string
}

func meshIface(cfg *Config) string {
	return cfg.WireGuardIface + "mesh"
}

func setupMesh(cfg *Config, logLevel string) error {
	if cfg.WireGuardMeshPort == 0 {
		INFO("mesh: no mesh port configured, mesh disabled")
		return nil
	}

	iface := meshIface(cfg)

	priv, err := loadOrGenerateLocalPrivKey(false)
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
	conf := fmt.Sprintf("public_key=%s\nreplace_allowed_ips=true\n", sanitizeIPC(pubKeyHex))
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

func fetchMesh(cfg *Config) (*types.WGMeshResponse, error) {
	if err := requireHTTPSControllerURL(cfg.ControllerURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.ControllerURL, "/")+"/wg/mesh", nil)
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

func reconcileMesh() {
	cfg := activeConfig.Load()
	if cfg == nil {
		return
	}

	resp, err := fetchMesh(cfg)
	if err != nil {
		WARN("mesh: fetch failed (keeping current peers): ", err)
		return
	}

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

	if wgMeshDevice == nil {
		return
	}

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
