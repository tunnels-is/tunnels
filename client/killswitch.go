package client

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// killSwitchMetric is the sink default. A lower metric wins, so the
	// tunnel default (metric 0 or 1) stays ahead of it and a physical
	// default is moved above it.
	killSwitchMetric = 50
	// killSwitchDemotedMetric is where a physical default sits while the
	// switch is on. It is worse than killSwitchMetric.
	killSwitchDemotedMetric = 100
)

var killSwitchMu sync.Mutex

var killSwitchWatchOnce sync.Once

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
	if want4 || want6 {
		startKillSwitchWatcher()
	}
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

func startKillSwitchWatcher() {
	killSwitchWatchOnce.Do(func() {
		go killSwitchWatchLoop()
	})
}

func killSwitchWatchLoop() {
	defer RecoverAndLog()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var stop <-chan struct{}
	if CancelContext != nil {
		stop = CancelContext.Done()
	} else {
		stop = make(chan struct{})
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cfg := CONFIG.Load()
			if cfg == nil {
				continue
			}
			_ = syncKillSwitch(cfg.KillSwitchIPv4, cfg.KillSwitchIPv6)
		}
	}
}

// demotePhysicalDefault reports whether a default route should be moved
// above the kill-switch sink. Narrower prefixes never reach this check.
// Tunnel defaults, the protect table, and the sink itself stay put.
func demotePhysicalDefault(isDefault, blackhole, tunnelLink, tunnelGateway, protectTable bool, priority int) bool {
	if !isDefault || blackhole || tunnelLink || tunnelGateway || protectTable {
		return false
	}
	return priority < killSwitchMetric
}

type defaultRouteChoice int

const (
	defaultChoiceNone defaultRouteChoice = iota
	defaultChoiceBlackhole
	defaultChoiceTunnel
	defaultChoicePhysical
)

// classifyDefaultRoute reads `route -n get default` output. A physical
// default is the only one the kill switch removes. A tunnel default and a
// blackhole stay.
func classifyDefaultRoute(out string, tunnels, gateways map[string]struct{}) defaultRouteChoice {
	if routeGetOutputHasBlackhole(out) {
		return defaultChoiceBlackhole
	}
	iface := routeGetValue(out, "interface:")
	gw := routeGetGateway(out)
	if iface == "" && gw == nil {
		return defaultChoiceNone
	}
	if _, ok := tunnels[iface]; ok {
		return defaultChoiceTunnel
	}
	if gw != nil {
		if _, ok := gateways[gw.String()]; ok {
			return defaultChoiceTunnel
		}
	}
	if gw == nil {
		return defaultChoiceNone
	}
	return defaultChoicePhysical
}

func routeGetOutputHasBlackhole(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "flags:") && strings.Contains(line, "BLACKHOLE") {
			return true
		}
	}
	return false
}

func routeGetValue(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, key))
	}
	return ""
}

// netstatHasBlackholeDefault reports a blackhole default in netstat -rn
// output. The B flag is RTF_BLACKHOLE.
func netstatHasBlackholeDefault(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "default", "0.0.0.0/0", "::/0", "0.0.0.0", "::":
		default:
			continue
		}
		for _, field := range fields[1:] {
			if routeFlagField(field) && strings.Contains(field, "B") {
				return true
			}
		}
	}
	return false
}

func routeFlagField(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}

func routeGetGateway(out string) net.IP {
	raw := routeGetValue(out, "gateway:")
	if raw == "" {
		return nil
	}
	token := strings.Fields(raw)[0]
	token, _, _ = strings.Cut(token, "%")
	ip := net.ParseIP(token)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return append(net.IP(nil), v4...)
	}
	return append(net.IP(nil), ip...)
}

// raisePhysicalDefault reports whether a Windows default route should be
// moved above the loopback sink. Prefix length 0 is the default. Loopback
// and tunnel defaults stay.
func raisePhysicalDefault(prefixLen, metric int, loopback, tunnel bool) bool {
	if prefixLen != 0 || loopback || tunnel {
		return false
	}
	return metric < killSwitchMetric
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
		if err := addIPv4Route(ip+"/32", ifName, gw4, "0"); err != nil {
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

func applyMissingKillSwitchDefaults(raw []byte, cfg *Config) {
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
