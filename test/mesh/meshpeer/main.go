// meshpeer is a minimal userspace WireGuard client used by the mesh
// integration test. It brings up a single wg0 peer pointed at a tunnels
// wg-server, assigns its controller-issued WireGuard IP, and routes the WAN
// CIDR through the tunnel — enough to exercise cross-server reachability
// (LazyBind provisioning + the server-to-server mesh) without the full GUI
// client. It then blocks until killed.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

func b64ToHex(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

func run(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %w: %s", args, err, string(out))
	}
	return nil
}

func main() {
	iface := flag.String("iface", "wg0", "wg interface name")
	privB64 := flag.String("priv", "", "client private key (base64)")
	serverPubB64 := flag.String("serverpub", "", "wg-server public key (base64)")
	endpoint := flag.String("endpoint", "", "wg-server endpoint host:port")
	wgIP := flag.String("ip", "", "this client's WireGuard IP (no mask)")
	allowed := flag.String("allowed", "", "AllowedIPs / routed CIDR (the WAN)")
	mtu := flag.Int("mtu", 1380, "tunnel MTU")
	flag.Parse()

	privHex, err := b64ToHex(*privB64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad private key:", err)
		os.Exit(1)
	}
	serverPubHex, err := b64ToHex(*serverPubB64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad server pubkey:", err)
		os.Exit(1)
	}

	tunDev, err := tun.CreateTUN(*iface, *mtu)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CreateTUN:", err)
		os.Exit(1)
	}
	logger := device.NewLogger(device.LogLevelError, "["+*iface+"] ")
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)

	conf := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s\npersistent_keepalive_interval=15\nallowed_ip=%s\n\n",
		privHex, serverPubHex, *endpoint, *allowed,
	)
	if err := dev.IpcSetOperation(bufio.NewReader(strings.NewReader(conf))); err != nil {
		fmt.Fprintln(os.Stderr, "IpcSet:", err)
		os.Exit(1)
	}
	if err := dev.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "device Up:", err)
		os.Exit(1)
	}

	// Assign the WG IP and route the WAN CIDR through the interface.
	if err := run("ip", "address", "add", *wgIP+"/32", "dev", *iface); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run("ip", "link", "set", *iface, "up"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run("ip", "route", "add", *allowed, "dev", *iface); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("meshpeer up: iface=%s ip=%s endpoint=%s allowed=%s\n", *iface, *wgIP, *endpoint, *allowed)
	select {} // block forever
}
