package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/version"
)

func (a *App) settingsPage() fyne.CanvasObject {
	cfg := a.config
	if cfg == nil {
		cfg = client.CloneConfig()
	}
	ver := version.Version
	api := version.ApiVersion
	if a.state != nil {
		if a.state.Version != "" {
			ver = a.state.Version
		}
		if a.state.APIVersion != 0 {
			api = a.state.APIVersion
		}
	}

	themes := newSegmented([]segItem{
		{themeTunnelsDark, "Dark"},
		{themeTunnels, "Light"},
		{themeSuzko, "Suzko"},
	}, live.name, func(key string) { a.setThemeName(key) })

	cards := []fyne.CanvasObject{
		card("Appearance", "",
			settingList(
				settingRow("Theme", "Colour theme for the whole app.", themes),
				settingRow("Zoom",
					fmt.Sprintf("Scales the entire interface. %s+ and %s- also work.", modKeyLabel(), modKeyLabel()),
					a.zoomStepper()),
			)),

		card("General", "",
			settingList(
				toggleRow("Advanced mode",
					"Show tunnels, the DNS resolver, statistics and system paths.",
					a.advanced, func(v bool) { a.setAdvanced(v) }),
				toggleRow("Bandwidth graphs",
					"Track per-tunnel throughput while connected.",
					cfg.BandwidthGraphs, func(bool) { a.toggleConfig("BandwidthGraphs") }),
			)),

		card("Kill switch",
			"Blackhole routes stay installed until you turn the switch off — including after disconnect or quitting the app.",
			settingList(
				toggleRow("Block IPv6",
					"On by default. Tunnels does not carry IPv6, so ::/0 is dropped and AAAA destinations cannot leak to your ISP.",
					cfg.KillSwitchIPv6, func(bool) { a.toggleConfig("KillSwitchIPv6") }),
				toggleRow("Block IPv4",
					"Off by default. When on, 0.0.0.0/0 is blackholed except the tunnel and pinned controller endpoints.",
					cfg.KillSwitchIPv4, func(bool) { a.toggleConfig("KillSwitchIPv4") }),
			)),

		card("Logging", "Which event types are captured.",
			settingList(
				toggleRow("Info", "", cfg.InfoLogging, func(bool) { a.toggleConfig("InfoLogging") }),
				toggleRow("Errors", "", cfg.ErrorLogging, func(bool) { a.toggleConfig("ErrorLogging") }),
				toggleRow("Debug", "Verbose internals. Noisy.", cfg.DebugLogging, func(bool) { a.toggleConfig("DebugLogging") }),
				toggleRow("Console", "Also write log lines to stdout.", cfg.ConsoleLogging, func(bool) { a.toggleConfig("ConsoleLogging") }),
				toggleRow("Terminal only", "Skip the log file entirely.", cfg.ConsoleLogOnly, func(bool) { a.toggleConfig("ConsoleLogOnly") }),
			)),
	}

	if a.advanced {
		cards = append(cards, card("DNS", "",
			toggleRow("Disable resolver",
				"The bundled DNS resolver is enabled by default.",
				cfg.DisableDNS, func(bool) { a.toggleConfig("DisableDNS") })))

		apiIP := kEntry("", cfg.APIIP)
		apiPort := kEntry("", cfg.APIPort)
		apiCert := kEntry("", cfg.APICert)
		apiKey := kEntry("", cfg.APIKey)
		apiDomains := kEntry("comma separated", strings.Join(cfg.APICertDomains, ","))
		apiIPs := kEntry("comma separated", strings.Join(cfg.APICertIPs, ","))
		saveAPI := primaryBtn("Save API server", func() {
			ip, port := strings.TrimSpace(apiIP.Text), strings.TrimSpace(apiPort.Text)
			cert, keyPath := strings.TrimSpace(apiCert.Text), strings.TrimSpace(apiKey.Text)
			domains, ips := splitCSV(apiDomains.Text), splitCSV(apiIPs.Text)
			a.updateConfig("Saving API server", func(c *client.Config) {
				c.APIIP, c.APIPort = ip, port
				c.APICert, c.APIKey = cert, keyPath
				c.APICertDomains, c.APICertIPs = domains, ips
			}, func() {
				a.note("API server saved")
				a.rebuild()
			})
		})

		cards = append(cards, card("API server",
			"Address the client listens on, plus an optional TLS certificate.",
			formRows(
				formPair(field("IP", apiIP), field("Port", apiPort)),
				field("Certificate domains", apiDomains),
				field("Certificate IPs", apiIPs),
				field("Certificate path", apiCert),
				field("Key path", apiKey),
				vspace(sp1),
				hstack(0, saveAPI),
			)))

		base, cfgPath, logPath, logFile := "", "", "", ""
		if a.state != nil && a.state.State != nil {
			st := a.state.State
			base = st.BasePath
			cfgPath = st.ConfigFileName
			logPath = st.LogPath
			logFile = st.LogFileName
		}
		cards = append(cards, fullRow(card("System", "Paths this instance is running with.",
			vstack(0,
				kvRow("Base path", base, true),
				kvRow("Config file", cfgPath, true),
				kvRow("Log path", logPath, true),
				kvRow("Log file", logFile, true),
			))))
	}

	sub := fmt.Sprintf("Tunnels %s · API v%d", ver, api)
	return pageShell("Settings", sub, nil, scrollBody(cards...))
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
