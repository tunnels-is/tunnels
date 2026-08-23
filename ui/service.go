package ui

import (
	"encoding/json"
	"errors"
	"time"

	"fyne.io/fyne/v2"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) note(msg string) { a.pushToast("ok", msg) }

func (a *App) fail(msg string) { a.pushToast("error", msg) }

func (a *App) pushToast(kind, msg string) {
	if msg == "" {
		return
	}
	a.toastKind = kind
	a.toastMsg = msg
	if a.toastBox != nil {
		a.toastBox.Objects = []fyne.CanvasObject{newToast(kind, msg)}
		a.toastBox.Refresh()
	}
	go func() {
		time.Sleep(4 * time.Second)
		a.uiDo(func() {
			if a.toastMsg != msg {
				return
			}
			a.toastMsg = ""
			if a.toastBox != nil {
				a.toastBox.Objects = nil
				a.toastBox.Refresh()
			}
		})
	}()
}

func (a *App) setThemeName(name string) {
	setLiveTheme(name)
	a.fyneApp.Preferences().SetString("ui-theme", name)
	a.rebuild()
	if a.side != nil {
		a.side.Refresh()
	}
}

func (a *App) refreshState() {
	st := client.GetFullState()
	a.state = st
	if st != nil {
		a.config = st.Config
		a.tunnels = st.Tunnels
		a.active = st.ActiveTunnels
	}
}

func (a *App) setUser(u *client.User) {
	a.user = u
	if u != nil && u.ID != "" {
		a.fyneApp.Preferences().SetString("activeUserID", u.ID)
		go func() {
			_ = client.SaveUser(u)
		}()
	}
	a.servers = nil
	a.devices = nil
	a.localDevices = nil
	a.serversLoaded = false
	a.devicesLoaded = false
}

func (a *App) clearSession() {
	a.user = nil
	a.servers = nil
	a.devices = nil
	a.localDevices = nil
	a.serversLoaded = false
	a.devicesLoaded = false
	a.fyneApp.Preferences().SetString("activeUserID", "")
}

func (a *App) logout(all bool) {
	if a.user == nil {
		a.clearSession()
		a.show(pageLogin)
		return
	}
	body := map[string]any{}
	if all {
		body["All"] = true
	} else if a.user.DeviceToken != nil {
		body["LogoutToken"] = a.user.DeviceToken.DT
	}
	user := a.user
	go func() {
		_, code, err := a.callController("/client/user/logout", body, true)
		a.uiDo(func() {
			if err != nil && code != 401 {
				a.fail("Unable to log out device")
			}
			if user.SaveFileHash != "" {
				_ = client.DeleteUser(user.SaveFileHash)
			}
			a.clearSession()
			a.note("Logged out")
			a.show(pageLogin)
		})
	}()
}

func (a *App) callController(path string, body any, auth bool) ([]byte, int, error) {
	return a.callControllerOn(a.controlServer(), path, body, auth)
}

func (a *App) controlServer() *client.ControlServer {
	if a.user != nil && a.user.ControlServer != nil {
		return a.user.ControlServer
	}
	return nil
}

func (a *App) callControllerOn(server *client.ControlServer, path string, body any, auth bool) ([]byte, int, error) {
	if server == nil {
		return nil, 0, errors.New("no control server found, please log in again")
	}
	uid, tok := "", ""
	if auth {
		if a.user == nil || a.user.DeviceToken == nil || a.user.DeviceToken.DT == "" {
			return nil, 401, errors.New("no auth token found, please log in again")
		}
		uid = a.user.ID
		tok = a.user.DeviceToken.DT
		if m, ok := body.(map[string]any); ok {
			m["UID"] = uid
			m["DeviceToken"] = tok
		}
	}
	raw, code, err := client.ControllerRequest(server, path, body, uid, tok)
	if err != nil {
		return raw, code, err
	}
	if code != 200 {
		return raw, code, errors.New(client.ControllerError(raw, "request failed"))
	}
	return raw, code, nil
}

func (a *App) fetchServers(force bool) {
	if a.user == nil {
		return
	}
	if a.serversFetching {
		return
	}
	if !force && a.serversLoaded {
		return
	}
	a.serversFetching = true
	go func() {
		raw, _, err := a.callController("/client/servers", map[string]any{"StartIndex": 0}, true)
		a.uiDo(func() {
			a.serversFetching = false
			a.serversLoaded = true
			if err != nil {
				a.fail("Unable to find servers")
				a.servers = []types.Server{}
				return
			}
			var list []types.Server
			if err := json.Unmarshal(raw, &list); err != nil {
				a.fail("Unable to parse servers")
				a.servers = []types.Server{}
				return
			}
			a.servers = list
			if len(list) == 0 {
				a.fail("Unable to find servers")
			}
			if a.current == pageServers || a.current == pageTunnels || a.current == pageDevices || a.current == pageTunnelEdit {
				a.reloadCurrent()
			}
		})
	}()
}

func (a *App) fetchDevices() {
	if a.user == nil {
		a.devices = nil
		a.localDevices = nil
		return
	}
	if a.devicesFetching {
		return
	}
	a.devicesFetching = true
	uid := a.user.ID
	go func() {
		raw, _, err := a.callController("/client/device/list/user", map[string]any{}, true)
		local, lerr := client.GetLocalDevices(uid)
		a.uiDo(func() {
			a.devicesFetching = false
			a.devicesLoaded = true
			if err != nil {
				a.devices = []types.Device{}
			} else {
				var list []types.Device
				if json.Unmarshal(raw, &list) == nil {
					a.devices = list
				} else {
					a.devices = []types.Device{}
				}
			}
			if lerr != nil {
				a.localDevices = nil
			} else {
				a.localDevices = local
			}
			if a.current == pageDevices {
				a.reloadCurrent()
			}
		})
	}()
}

func (a *App) activeByServer() map[string]*client.TUN {
	out := map[string]*client.TUN{}
	if a.user == nil {
		return out
	}
	for _, t := range a.active {
		if t != nil && t.CR != nil && t.CR.UserID == a.user.ID && t.CR.ServerID != "" {
			out[t.CR.ServerID] = t
		}
	}
	return out
}

func (a *App) activeByTag() map[string]*client.TUN {
	out := map[string]*client.TUN{}
	if a.user == nil {
		return out
	}
	for _, t := range a.active {
		if t != nil && t.CR != nil && t.CR.UserID == a.user.ID && t.CR.Tag != "" {
			out[t.CR.Tag] = t
		}
	}
	return out
}

func (a *App) serverByID(id string) *types.Server {
	for i := range a.servers {
		if a.servers[i].ID.String() == id {
			return &a.servers[i]
		}
	}
	return nil
}

func (a *App) connectToServer(s types.Server) {
	if a.user == nil || a.user.DeviceToken == nil {
		a.fail("You are not logged in")
		a.show(pageLogin)
		return
	}
	a.note("Connecting...")
	req := &client.ConnectionRequest{
		UserID:      a.user.ID,
		DeviceToken: a.user.DeviceToken.DT,
		ServerID:    s.ID.String(),
		Server:      a.user.ControlServer,
	}
	go func() {
		_, err := client.ServerConnect(req)
		a.uiDo(func() {
			if err != nil {
				a.fail(err.Error())
			} else {
				a.note("Connection ready")
			}
			a.refreshState()
			a.reloadCurrent()
		})
	}()
}

func (a *App) connectTunnel(meta *client.TunnelMETA) {
	if a.user == nil || a.user.DeviceToken == nil {
		a.fail("You are not logged in")
		return
	}
	server := a.serverByID(meta.ServerID)
	if server == nil {
		a.fail("Unable to find server with the given ID")
		return
	}
	a.note("Connecting...")
	req := &client.ConnectionRequest{
		UserID:      a.user.ID,
		DeviceToken: a.user.DeviceToken.DT,
		Tag:         meta.Tag,
		ServerID:    server.ID.String(),
		Server:      a.user.ControlServer,
	}
	go func() {
		_, err := client.PublicConnect(req)
		a.uiDo(func() {
			if err != nil {
				a.fail(err.Error())
			} else {
				a.note("Connection ready")
			}
			a.refreshState()
			a.reloadCurrent()
		})
	}()
}

func (a *App) disconnectActive(t *client.TUN) {
	if t == nil {
		return
	}
	tag := ""
	if t.CR != nil {
		tag = t.CR.Tag
	}
	a.note("Disconnecting...")
	id := t.ID
	go func() {
		err := client.DisconnectTunnel(id, tag)
		a.uiDo(func() {
			if err != nil {
				a.fail(err.Error())
			} else {
				if tag == "" {
					tag = "tunnel"
				}
				a.note("Disconnected from " + tag)
			}
			a.refreshState()
			a.reloadCurrent()
		})
	}()
}

func (a *App) saveConfig(cfg *client.Config) bool {
	if cfg == nil {
		return false
	}
	if err := client.SetConfig(cfg); err != nil {
		a.fail(err.Error())
		return false
	}
	a.refreshState()
	a.note("Config saved")
	return true
}

func (a *App) toggleConfig(key string) {
	cfg := client.CloneConfig()
	switch key {
	case "InfoLogging":
		cfg.InfoLogging = !cfg.InfoLogging
	case "ErrorLogging":
		cfg.ErrorLogging = !cfg.ErrorLogging
	case "ConsoleLogging":
		cfg.ConsoleLogging = !cfg.ConsoleLogging
	case "DebugLogging":
		cfg.DebugLogging = !cfg.DebugLogging
	case "ConsoleLogOnly":
		cfg.ConsoleLogOnly = !cfg.ConsoleLogOnly
	case "BandwidthGraphs":
		cfg.BandwidthGraphs = !cfg.BandwidthGraphs
	case "KillSwitchIPv6":
		cfg.KillSwitchIPv6 = !cfg.KillSwitchIPv6
	case "KillSwitchIPv4":
		cfg.KillSwitchIPv4 = !cfg.KillSwitchIPv4
	case "DisableDNS":
		cfg.DisableDNS = !cfg.DisableDNS
	case "DNSOverHTTPS":
		cfg.DNSOverHTTPS = !cfg.DNSOverHTTPS
	case "LogBlockedDomains":
		cfg.LogBlockedDomains = !cfg.LogBlockedDomains
	case "LogAllDomains":
		cfg.LogAllDomains = !cfg.LogAllDomains
	case "DNSstats":
		cfg.DNSstats = !cfg.DNSstats
	case "DNSHTTPSAutomatic":
		cfg.DNSHTTPSAutomatic = !cfg.DNSHTTPSAutomatic
	default:
		return
	}
	a.saveConfig(cfg)
}

func (a *App) startLogPump() {
	ch := make(chan string, 256)
	client.SetUILogHandler(func(line string) {
		if len(line) == 0 {
			return
		}
		select {
		case ch <- line:
		default:
		}
	})
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		pending := make([]string, 0, 64)
		for {
			select {
			case line := <-ch:
				pending = append(pending, line)
				if len(pending) > 200 {
					pending = pending[len(pending)-200:]
				}
			case <-tick.C:
				if len(pending) == 0 {
					continue
				}
				batch := pending
				pending = make([]string, 0, 64)
				a.uiDo(func() {
					a.logs = append(a.logs, batch...)
					if len(a.logs) > 1000 {
						a.logs = append([]string(nil), a.logs[len(a.logs)-800:]...)
					}
					if a.current == pageLogs {
						a.paintLogs()
					}
				})
			}
		}
	}()
}

// paintLogs refreshes the log list in place. The list is virtualised, so the
// whole buffer can be bound without the old 200-line truncation.
func (a *App) paintLogs() {
	a.recomputeLogView()
	if a.logList == nil {
		// The page is showing its empty state; rebuild so the list appears.
		if len(a.logView) > 0 {
			a.reloadCurrent()
		}
		return
	}
	a.logList.Refresh()
}

func sameSession(aTok, bTok *client.DEVICE_TOKEN) bool {
	if aTok == nil || bTok == nil {
		return false
	}
	if aTok.DT != "" && bTok.DT != "" && aTok.DT == bTok.DT {
		return true
	}
	if aTok.N == "" || bTok.N == "" || aTok.N != bTok.N {
		return false
	}
	return aTok.Created.Equal(bTok.Created)
}
