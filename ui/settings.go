package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/version"
)

func (a *App) settingsPage() fyne.CanvasObject {
	cfg := a.config
	if cfg == nil {
		cfg = client.CloneConfig()
	}
	ver := version.Version
	if a.state != nil && a.state.Version != "" {
		ver = a.state.Version
	}

	adv := bindCheck("Advanced mode", a.advanced, func(v bool) { a.setAdvanced(v) })
	bw := bindCheck("Bandwidth graphs", cfg.BandwidthGraphs, func(v bool) { a.toggleConfig("BandwidthGraphs") })

	ks6 := bindCheck("IPv6 kill switch", cfg.KillSwitchIPv6, func(v bool) { a.toggleConfig("KillSwitchIPv6") })
	ks4 := bindCheck("IPv4 kill switch", cfg.KillSwitchIPv4, func(v bool) { a.toggleConfig("KillSwitchIPv4") })

	logChecks := container.NewVBox()
	addLog := func(key, label string, v bool) {
		logChecks.Add(bindCheck(label, v, func(_ bool) { a.toggleConfig(key) }))
	}
	addLog("InfoLogging", "Info", cfg.InfoLogging)
	addLog("ErrorLogging", "Errors", cfg.ErrorLogging)
	addLog("ConsoleLogging", "Console", cfg.ConsoleLogging)
	addLog("DebugLogging", "Debug", cfg.DebugLogging)
	addLog("ConsoleLogOnly", "Terminal only", cfg.ConsoleLogOnly)

	themeNames := []string{"Tunnels Dark", "Tunnels Light", "Suzko"}
	themeVals := map[string]string{
		"Tunnels Dark":  themeTunnelsDark,
		"Tunnels Light": themeTunnels,
		"Suzko":         themeSuzko,
	}
	curLabel := "Tunnels Dark"
	switch live.name {
	case themeTunnels:
		curLabel = "Tunnels Light"
	case themeSuzko:
		curLabel = "Suzko"
	}
	themeSel := widget.NewSelect(themeNames, func(label string) {
		if v, ok := themeVals[label]; ok {
			a.setThemeName(v)
		}
	})
	themeSel.SetSelected(curLabel)

	objs := []fyne.CanvasObject{
		caption(fmt.Sprintf("App %s  ·  API %d", ver, version.ApiVersion)),
		card("Appearance", "Select a color theme.", container.NewVBox(
			kLabeled("Theme", kInput(themeSel)),
		)),
		card("Advanced", "Show advanced configuration: API server, DNS and system details.", container.NewVBox(adv, bw)),
		card("Kill switch", "Blackhole routes stay installed until you turn the switch off — including after disconnect or quitting the app.", container.NewVBox(
			ks6, muted("On by default. Tunnels does not put IPv6 in the tunnel; this drops ::/0 so AAAA destinations cannot leak to the ISP."),
			ks4, muted("Off by default. When on, 0.0.0.0/0 is blackholed except the tunnel and pinned controller/VPN endpoints."),
		)),
		card("Logging", "Select which event types are captured.", logChecks),
	}

	if a.advanced {
		disableDNS := bindCheck("Disable DNS", cfg.DisableDNS, func(v bool) { a.toggleConfig("DisableDNS") })
		objs = append(objs, card("DNS", "The local DNS resolver is enabled by default.", disableDNS))

		apiIP := widget.NewEntry()
		apiIP.SetText(cfg.APIIP)
		apiPort := widget.NewEntry()
		apiPort.SetText(cfg.APIPort)
		apiCert := widget.NewEntry()
		apiCert.SetText(cfg.APICert)
		apiKey := widget.NewEntry()
		apiKey.SetText(cfg.APIKey)
		apiDomains := widget.NewEntry()
		apiDomains.SetText(strings.Join(cfg.APICertDomains, ","))
		apiIPs := widget.NewEntry()
		apiIPs.SetText(strings.Join(cfg.APICertIPs, ","))
		saveAPI := primaryBtn("Save API server", func() {
			next := client.CloneConfig()
			next.APIIP = strings.TrimSpace(apiIP.Text)
			next.APIPort = strings.TrimSpace(apiPort.Text)
			next.APICert = strings.TrimSpace(apiCert.Text)
			next.APIKey = strings.TrimSpace(apiKey.Text)
			next.APICertDomains = splitCSV(apiDomains.Text)
			next.APICertIPs = splitCSV(apiIPs.Text)
			if a.saveConfig(next) {
				a.rebuild()
			}
		})
		objs = append(objs, card("API server", "Address the client listens on, plus optional TLS certificate.", container.NewVBox(
			kLabeled("IP", kInput(apiIP)), kLabeled("Port", kInput(apiPort)),
			kLabeled("Cert domains", kInput(apiDomains)), kLabeled("Cert IPs", kInput(apiIPs)),
			kLabeled("Cert path", kInput(apiCert)), kLabeled("Key path", kInput(apiKey)),
			vspace(8),
			saveAPI,
		)))

		base, cfgPath, logPath, logFile := "", "", "", ""
		if a.state != nil && a.state.State != nil {
			st := a.state.State
			base = st.BasePath
			cfgPath = st.ConfigFileName
			logPath = st.LogPath
			logFile = st.LogFileName
		}
		objs = append(objs, card("System", "Paths, files and privileges this app is running with.", container.NewVBox(
			infoRow("Base path", base),
			infoRow("Config", cfgPath),
			infoRow("Log path", logPath),
			infoRow("Log file", logFile),
		)))
	}

	return pageScroll(objs...)
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
