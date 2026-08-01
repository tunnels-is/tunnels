package wgserver

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	wgDevice   *device.Device
	wgLazyBind *LazyBind

	addedPeerKeys sync.Map
)

func wgDeviceLogLevel(level string) int {
	switch level {
	case "silent":
		return device.LogLevelSilent
	case "error", "warn":
		return device.LogLevelError
	case "info", "debug":
		return device.LogLevelVerbose
	default:
		return device.LogLevelVerbose
	}
}

func setupWireGuard(cfg *Config, logLevel string) error {
	wgLogger := device.NewLogger(wgDeviceLogLevel(logLevel), "[wg] ")

	tunDev, err := tun.CreateTUN(cfg.WireGuardIface, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("CreateTUN %q: %w", cfg.WireGuardIface, err)
	}

	if err := initPeerList(cfg.WireGuardSubnet, cfg.WireGuardSubnet6); err != nil {
		return fmt.Errorf("init peer list: %w", err)
	}
	startFlowCleaner()

	tunInterface, err := newInspectingTUN(tunDev, cfg)
	if err != nil {
		return fmt.Errorf("inspector setup: %w", err)
	}
	if cfg.EnableFirewall {
		INFO("firewall enabled on ", cfg.WireGuardIface,
			" — peer-to-peer ingress denied by default, control port udp/", aclControlPort)
	} else {
		INFO("firewall disabled on ", cfg.WireGuardIface,
			" — peer-to-peer traffic unrestricted, server WG IP blocked for peers")
	}

	if len(cfg.WireGuardPrivKey) != 32 {
		return fmt.Errorf("invalid WireGuardPrivKey: expected 32 bytes, got %d", len(cfg.WireGuardPrivKey))
	}
	pubBytes, err := curve25519.X25519(cfg.WireGuardPrivKey, curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("derive pubkey for LazyBind: %w", err)
	}

	privCopy := make([]byte, 32)
	copy(privCopy, cfg.WireGuardPrivKey)

	wgLazyBind = NewLazyBind(conn.NewDefaultBind(), privCopy, pubBytes, cfg.HandshakeBufferSize, cfg.HandshakeRatePerIP)
	wgDevice = device.NewDevice(tunInterface, wgLazyBind, wgLogger)

	privKeyHex := make([]byte, hex.EncodedLen(32))
	hex.Encode(privKeyHex, cfg.WireGuardPrivKey)

	conf := fmt.Appendf(nil, "private_key=%s\nlisten_port=%d\n\n", privKeyHex, cfg.WireGuardPort)
	zeroBytes(privKeyHex)

	if err := ipcSetBytes(conf); err != nil {
		zeroBytes(conf)
		return fmt.Errorf("IpcSet device config: %w", err)
	}
	zeroBytes(conf)

	if err := wgDevice.Up(); err != nil {
		return fmt.Errorf("device Up: %w", err)
	}

	pubKeyB64, err := derivePubKey(cfg.WireGuardPrivKey)
	if err != nil {
		return fmt.Errorf("derive pubkey: %w", err)
	}

	zeroBytes(cfg.WireGuardPrivKey)

	INFO("wireguard device started, pubkey=", pubKeyB64, " port=", cfg.WireGuardPort)
	return nil
}

func sanitizeIPC(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func AddPeer(pubKeyHex string, allowedIPs ...string) error {
	conf := fmt.Sprintf("public_key=%s\n", sanitizeIPC(pubKeyHex))
	for _, aip := range allowedIPs {
		conf += fmt.Sprintf("allowed_ip=%s\n", sanitizeIPC(aip))
	}
	conf += "\n"
	return ipcSet(conf)
}

func RemovePeer(pubKeyHex string) error {
	conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", sanitizeIPC(pubKeyHex))
	return ipcSet(conf)
}

func ipcSet(conf string) error {
	if wgDevice == nil {
		return fmt.Errorf("wireguard device not initialized")
	}
	return wgDevice.IpcSetOperation(bufio.NewReader(strings.NewReader(conf)))
}

func ipcSetBytes(conf []byte) error {
	if wgDevice == nil {
		return fmt.Errorf("wireguard device not initialized")
	}
	return wgDevice.IpcSetOperation(bufio.NewReader(bytes.NewReader(conf)))
}

func b64ToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return hex.EncodeToString(b), nil
}
