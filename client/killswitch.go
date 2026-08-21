package client

import (
	"bytes"
	"encoding/json"
	"net"
	"sync"

	"gopkg.in/yaml.v3"
)

var killSwitchMu sync.Mutex

// applyConfiguredKillSwitch installs or removes IPv4/IPv6 blackhole
// defaults to match CONFIG. Routes stay until the matching setting is
// turned off — not on disconnect, crash, or process exit.
func applyConfiguredKillSwitch() error {
	cfg := CONFIG.Load()
	if cfg == nil {
		return nil
	}
	return syncKillSwitch(cfg.KillSwitchIPv4, cfg.KillSwitchIPv6)
}

func syncKillSwitch(want4, want6 bool) error {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()

	var first error
	if want4 {
		if err := enableKillSwitchIPv4(); err != nil {
			first = err
			SECURITY("IPv4 kill switch could not be applied: ", err)
		} else {
			pinControlPlaneIPv4Routes()
		}
	} else {
		disableKillSwitchIPv4()
	}

	if want6 {
		if err := enableKillSwitchIPv6(); err != nil && first == nil {
			first = err
			SECURITY("IPv6 kill switch could not be applied: ", err)
		}
	} else {
		disableKillSwitchIPv6()
	}

	// Reinstall the WireGuard protect table / host /32s / socket pin if a
	// tunnel is up. The IPv4 blackhole is in the main table; marked WG
	// packets use table wgProtectTable and must not be left pointing at a
	// stale default after demoteLowMetricDefaults.
	refreshEndpointProtect()
	return first
}

func pinControlPlaneIPv4Routes() {
	loadDefaultInterface()
	gw := loadDefaultGateway()
	if gw == nil || gw.To4() == nil {
		DEBUG("kill switch: no default gateway; controller /32 pins skipped")
		return
	}
	gw4 := gw.To4().String()

	ifName := ""
	if n := STATE.Load().DefaultInterfaceName.Load(); n != nil {
		ifName = *n
	}

	seen := map[string]struct{}{}
	pin := func(ip string) {
		if ip == "" {
			return
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			return
		}
		ip = parsed.To4().String()
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		if err := IP_AddRoute(ip+"/32", ifName, gw4, "0"); err != nil {
			DEBUG("kill switch: pin ", ip, ": ", err)
		}
	}

	pin(DefaultControllerIP)
	for _, h := range collectProtectHosts() {
		pin(h)
	}
	cfg := CONFIG.Load()
	if cfg == nil {
		return
	}
	for _, cs := range cfg.ControlServers {
		if cs == nil || cs.Host == "" {
			continue
		}
		if ip := net.ParseIP(cs.Host); ip != nil {
			pin(ip.String())
			continue
		}
		addrs, err := net.LookupHost(cs.Host)
		if err != nil {
			DEBUG("kill switch: resolve ", cs.Host, ": ", err)
			continue
		}
		for _, a := range addrs {
			pin(a)
		}
	}
}

func applyMissingKillSwitchDefaults(raw []byte, cfg *configV2) {
	if cfg == nil || len(raw) == 0 {
		return
	}
	keys, ok := configObjectKeys(raw)
	if !ok {
		return
	}
	if _, present := keys["KillSwitchIPv6"]; !present {
		cfg.KillSwitchIPv6 = true
	}
}

func configObjectKeys(raw []byte) (map[string]struct{}, bool) {
	raw = bytes.TrimSpace(raw)
	var probe map[string]any
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, false
		}
	} else if err := yaml.Unmarshal(raw, &probe); err != nil {
		return nil, false
	}
	out := make(map[string]struct{}, len(probe))
	for k := range probe {
		out[k] = struct{}{}
	}
	return out, true
}
