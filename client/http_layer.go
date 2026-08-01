package client

import (
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/version"
	"golang.org/x/net/websocket"
)

var (
	discardLogger   = log.New(io.Discard, "", 0)
	httpErrorLogger = log.New(os.Stdout, "", 0)
	sessionToken    string
)

func LaunchAPI() {
	defer RecoverAndLog()

	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		ERROR("failed to generate session token: ", err)
		return
	}
	sessionToken = hex.EncodeToString(tokenBytes)

	assetHandler := http.FileServer(getFileSystem())

	mux := http.NewServeMux()
	mux.HandleFunc("/logs", handleWebSocketAuth)
	mux.Handle("/", withSessionCookie(assetHandler))
	mux.Handle("/assets/", withSessionCookie(assetHandler))
	mux.HandleFunc("/v1/method/{method}", HTTPhandler)
	API_SERVER = http.Server{
		Handler: mux,
	}
	if EnableTLS {
		API_SERVER.TLSConfig = makeTLSConfig()
	}
	state := STATE.Load()
	if state.Debug {
		API_SERVER.ErrorLog = httpErrorLogger
	} else {
		API_SERVER.ErrorLog = discardLogger
	}

	conf := CONFIG.Load()

	ip := conf.APIIP
	if ip == "" {
		ip = DefaultAPIIP
	}

	port := conf.APIPort
	if port == "" {
		port = DefaultAPIPort
	}

	API_SERVER.Addr = ip + ":" + port
	ln, err := net.Listen("tcp4", API_SERVER.Addr)
	if err != nil {
		ERROR("api start error: ", err)
		return
	}

	INFO("====== API SERVER =========")
	INFO("ADDR: ", ln.Addr())
	INFO("IP: ", ip)
	INFO("PORT: ", port)
	INFO("Key: ", conf.APIKey)
	INFO("Cert: ", conf.APICert)
	// The session token is deliberately NOT logged — anything that can read
	// the log would gain a valid local-API session.
	if DevMode {
		SECURITY("DEV MODE ENABLED: local API authentication is DISABLED and credentialed CORS is open — never use this in a shipped/production build")
	}
	INFO("===========================")

	if EnableTLS {
		if err := API_SERVER.ServeTLS(ln, conf.APICert, conf.APIKey); err != http.ErrServerClosed {
			ERROR("api start error: ", err)
		}
	} else {
		if err := API_SERVER.Serve(ln); err != http.ErrServerClosed {
			ERROR("api start error: ", err)
		}
	}
}

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(DIST_EMBED, "dist")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func makeTLSConfig() (tc *tls.Config) {
	conf := CONFIG.Load()
	tc = new(tls.Config)
	tc.InsecureSkipVerify = true

	certsExist := true
	_, err := os.Stat(conf.APICert)
	if err != nil {
		certsExist = false
	}
	_, err = os.Stat(conf.APIKey)
	if err != nil {
		certsExist = false
	}

	if !certsExist {
		_, err := certs.MakeCert(
			conf.APICertType,
			conf.APICert,
			conf.APIKey,
			conf.APICertIPs,
			conf.APICertDomains,
			"",
			time.Time{},
			true,
		)
		if err != nil {
			ERROR("Certificate error:", err)
			return
		}
	}
	return
}

func setSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "X-Session-Token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   EnableTLS,
	})
}

func withSessionCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only hand the session token to a genuinely local caller. A DNS-rebound
		// origin (evil.com→127.0.0.1) presents its own Host header (browsers
		// forbid forging Host via fetch), so this refuses to bootstrap it.
		if isLocalRequest(r) {
			setSessionCookie(w)
		}
		next.ServeHTTP(w, r)
	})
}

func checkSessionToken(r *http.Request) bool {
	cookie, err := r.Cookie("X-Session-Token")
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sessionToken)) == 1
}

// isLocalRequest reports whether the request's Host header names a loopback
// address (or the Wails webview host). This is the anti-DNS-rebinding gate: an
// attacker page rebinding its domain to 127.0.0.1 still sends `Host: evil.com`,
// which fails here — a browser will not let script forge the Host header.
func isLocalRequest(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]") // strip IPv6 brackets
	switch strings.ToLower(host) {
	case "localhost", "wails.localhost", "wails":
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return true
	}
	// Honor a deliberate non-loopback API bind so remote UI access an operator
	// explicitly configured isn't broken. A wildcard bind (0.0.0.0/::) means
	// "remote access opted in", so the Host check can't add value there; a
	// concrete bind IP is matched exactly (a rebound evil.com Host still fails).
	if conf := CONFIG.Load(); conf != nil && conf.APIIP != "" {
		if bindIP := net.ParseIP(conf.APIIP); bindIP != nil {
			if bindIP.IsUnspecified() {
				return true
			}
			if ip != nil && ip.Equal(bindIP) {
				return true
			}
		}
	}
	return false
}

func HTTPhandler(w http.ResponseWriter, r *http.Request) {
	// Reject anything not addressed to a loopback Host, regardless of auth —
	// the primary defense against DNS rebinding driving the local API.
	if !DevMode && !isLocalRequest(r) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		r.Body.Close()
		return
	}
	if DevMode {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			r.Body.Close()
			return
		}
	} else if !checkSessionToken(r) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		r.Body.Close()
		return
	}

	method := r.PathValue("method")
	switch method {
	case "connect":
		HTTP_Connect(w, r)
		return
	case "connectServer":
		HTTP_ServerConnect(w, r)
		return
	case "autoConnect":
		HTTP_AutoConnect(w, r)
		return
	case "disconnect":
		HTTP_Disconnect(w, r)
		return
	case "resetNetwork":
		HTTP_ResetNetwork(w, r)
		return
	case "getQRCode":
		HTTP_GetQRCode(w, r)
		return
	case "forwardToController":
		HTTP_ForwardToController(w, r)
		return
	case "createTunnel":
		HTTP_CreateTunnel(w, r)
		return
	case "deleteTunnel":
		HTTP_DeleteTunnels(w, r)
		return
	case "setUser":
		HTTP_SetUser(w, r)
		return
	case "getUsers":
		HTTP_GetUsers(w, r)
		return
	case "delUser":
		HTTP_DelUser(w, r)
		return
	case "getState":
		HTTP_GetState(w, r)
		return
	case "setConfig":
		HTTP_SetConfig(w, r)
		return
	case "setTunnel":
		HTTP_SetTunnel(w, r)
		return
	case "setTunnelPeers":
		HTTP_SetTunnelPeers(w, r)
		return
	case "getDNSStats":
		HTTP_GetDNSStats(w, r)
		return
	case "getLogs":
		HTTP_GetLogs(w, r)
		return
	case "createDeviceWithKeys":
		HTTP_CreateDeviceWithKeys(w, r)
		return
	default:
	}

	w.WriteHeader(200)
	r.Body.Close()
}

var LogSocket atomic.Pointer[websocket.Conn]

func handleWebSocketAuth(w http.ResponseWriter, r *http.Request) {
	if !DevMode && (!isLocalRequest(r) || !checkSessionToken(r)) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		return
	}
	s := websocket.Server{Handler: handleWebSocket}
	s.ServeHTTP(w, r)
}

func handleWebSocket(ws *websocket.Conn) {
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()
	defer RecoverAndLog()

	LogSocket.Store(ws)
	for event := range APILogQueue {
		err := websocket.Message.Send(ws, event)
		if err != nil {

			gws := LogSocket.Load()
			if gws != nil {
				_ = websocket.Message.Send(gws, event)
			}
			return
		}
	}
}

func Bind[I any](form I, r *http.Request) (err error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 2<<20) // 2MB
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(form)
	return
}

func STRING(w http.ResponseWriter, r *http.Request, code int, data string) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(data))
}

func JSON(w http.ResponseWriter, r *http.Request, code int, data any) {
	if data == nil {
		if code == 0 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(code)
		}
		return
	}
	defer RecoverAndLog()
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()

	outb, err := json.Marshal(data)
	if err != nil {
		ERROR("Unable to write encoded json to response writer:", err)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(code)
	_, _ = w.Write(outb)
}

type StateResponse struct {
	Version       string
	APIVersion    int
	Timezone      string
	Config        *configV2
	State         *stateV2
	Tunnels       []*TunnelMETA
	ActiveTunnels []*TUN
	Network       StateNetworkResponse
}

// getSystemTimezone returns the host's IANA timezone name, best-effort.
// Empty when undetectable (the UI falls back to the webview Intl API).
func getSystemTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i != -1 {
			return link[i+len("zoneinfo/"):]
		}
	}
	return ""
}

type StateNetworkResponse struct {
	DefaultGateway       net.IP
	DefaultInterface     net.IP
	DefaultInterfaceID   int32
	DefaultInterfaceName string
}

func HTTP_GetLogs(w http.ResponseWriter, r *http.Request) {
	PollLogMu.Lock()
	logs := PollLogBuf
	PollLogBuf = nil
	PollLogMu.Unlock()
	JSON(w, r, 200, logs)
}

func HTTP_GetDNSStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]any)
	DNSStatsMap.Range(func(key string, value any) bool {
		stats[key] = value
		return true
	})
	JSON(w, r, 200, stats)
}

func HTTP_GetState(w http.ResponseWriter, r *http.Request) {
	JSON(w, r, 200, GetFullState())
}

func HTTP_SetUser(w http.ResponseWriter, r *http.Request) {
	u := new(User)
	err := Bind(u, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	err = saveUser(u)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, u)
}

func HTTP_DelUser(w http.ResponseWriter, r *http.Request) {
	u := new(DelUserForm)
	err := Bind(u, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, delUser(u.Hash))
}

func HTTP_GetUsers(w http.ResponseWriter, r *http.Request) {
	u, err := getUsers()
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, u)
}

func GetFullState() (s *StateResponse) {
	defer RecoverAndLog()
	state := STATE.Load()
	s = new(StateResponse)
	s.Version = version.Version
	s.APIVersion = version.ApiVersion
	s.Timezone = getSystemTimezone()
	s.Config = CONFIG.Load()
	s.State = state

	s.Network.DefaultInterfaceID = state.DefaultInterfaceID.Load()
	defInt := state.DefaultInterface.Load()
	if defInt != nil {
		s.Network.DefaultInterface = *defInt
	}
	defIntName := state.DefaultInterfaceName.Load()
	if defIntName != nil {
		s.Network.DefaultInterfaceName = *defIntName
	}
	defGate := state.DefaultGateway.Load()
	if defGate != nil {
		s.Network.DefaultGateway = *defGate
	}

	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		s.Tunnels = append(s.Tunnels, tun)
		return true
	})

	tunnelMapRange(func(tun *TUN) bool {
		s.ActiveTunnels = append(s.ActiveTunnels, tun)
		return true
	})
	return
}

func HTTP_Connect(w http.ResponseWriter, r *http.Request) {
	ns := new(ConnectionRequest)
	err := Bind(ns, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	code, err := PublicConnect(ns)
	if err != nil {
		STRING(w, r, code, err.Error())
		return
	}
	JSON(w, r, code, nil)
}

// HTTP_ServerConnect connects to a chosen server using the default tunnel,
// reconciling that tunnel's device to the target server first.
func HTTP_ServerConnect(w http.ResponseWriter, r *http.Request) {
	ns := new(ConnectionRequest)
	err := Bind(ns, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	code, err := ServerConnect(ns)
	if err != nil {
		STRING(w, r, code, err.Error())
		return
	}
	JSON(w, r, code, nil)
}

func HTTP_AutoConnect(w http.ResponseWriter, r *http.Request) {
	form := new(AutoConnectForm)
	err := Bind(form, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	resp, code, err := CountryAutoConnect(form)
	if err != nil {
		STRING(w, r, code, err.Error())
		return
	}
	JSON(w, r, code, resp)
}

func HTTP_Disconnect(w http.ResponseWriter, r *http.Request) {
	DF := new(DisconnectForm)
	err := Bind(DF, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	// A user-initiated disconnect must stop any auto-reconnect loop for this
	// tunnel and release its kill switch — otherwise a tunnel that dropped (kill
	// switch engaged, reconnect looping) would keep re-connecting and the
	// blackhole route would strand it offline. Prefer the caller-supplied stable
	// Tag; fall back to resolving it from the live tunnel by ID. Only if neither
	// is available do we stop all loops (last resort — a mid-reconnect tunnel we
	// can't otherwise name).
	tag := DF.Tag
	if tag == "" {
		tunnelMapRange(func(t *TUN) bool {
			if t.ID == DF.ID {
				if m := t.meta.Load(); m != nil {
					tag = m.Tag
				}
				return false
			}
			return true
		})
	}
	if tag != "" {
		stopReconnect(tag)
		releaseKillSwitch(tag)
	} else {
		stopAllReconnects()
		releaseAllKillSwitches()
	}

	err = Disconnect(DF.ID, false)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, nil)
}

func HTTP_ResetNetwork(w http.ResponseWriter, r *http.Request) {
	ResetEverything()
	JSON(w, r, 200, nil)
}

type saveTunnelForm struct {
	Meta   *TunnelMETA
	OldTag string
}

func HTTP_SetTunnel(w http.ResponseWriter, r *http.Request) {
	newForm := new(saveTunnelForm)
	err := Bind(newForm, r)
	if err != nil {
		JSON(w, r, 400, err.Error())
		return
	}

	isConnected := false
	tunnelMapRange(func(t *TUN) bool {
		if t.CR != nil {
			if t.CR.Tag == newForm.Meta.Tag {
				isConnected = true
				return false
			}
		}
		return true
	})
	if isConnected {
		JSON(w, r, 400, "tunnel is connected")
		return
	}

	errors := validateTunnelMeta(newForm.Meta, newForm.OldTag)
	if len(errors) > 0 {
		JSON(w, r, 400, errors)
		return
	}

	TunnelMetaMap.Store(newForm.Meta.Tag, newForm.Meta)
	err = writeTunnelsToDisk(newForm.Meta.Tag)
	if err != nil {
		JSON(w, r, 400, err.Error())
		return
	}

	if newForm.OldTag != newForm.Meta.Tag {
		TunnelMetaMap.Delete(newForm.OldTag)
		state := STATE.Load()
		ext := newForm.Meta.ConfigFormat
		if ext == "" {
			ext = tunnelFileSuffix
		}
		err = os.Remove(state.TunnelsPath + newForm.OldTag + ext)
		if err != nil {
			JSON(w, r, 400, err.Error())
			return
		}
	}

	JSON(w, r, 200, nil)
}

type setTunnelPeersForm struct {
	Tag          string
	AllowedHosts []string
	AllowAll     bool
}

// HTTP_SetTunnelPeers replaces a tunnel's firewall policy (AllowedHosts plus
// the AllowAll flag), persists the meta, and announces it to the wg-server
// when the tunnel is connected. Unlike setTunnel, this works on connected
// tunnels.
func HTTP_SetTunnelPeers(w http.ResponseWriter, r *http.Request) {
	form := new(setTunnelPeersForm)
	if err := Bind(form, r); err != nil {
		JSON(w, r, 400, err.Error())
		return
	}

	meta, ok := TunnelMetaMap.Load(form.Tag)
	if !ok {
		JSON(w, r, 404, "tunnel not found")
		return
	}

	seen := make(map[string]struct{}, len(form.AllowedHosts))
	hosts := make([]string, 0, len(form.AllowedHosts))
	for _, h := range form.AllowedHosts {
		entry, err := NormalizeAllowedHost(h)
		if err != nil {
			JSON(w, r, 400, err.Error())
			return
		}
		if _, dup := seen[entry]; dup {
			continue
		}
		seen[entry] = struct{}{}
		hosts = append(hosts, entry)
	}

	meta.AllowedHosts = hosts
	meta.AllowAll = form.AllowAll
	TunnelMetaMap.Store(meta.Tag, meta)
	if err := writeTunnelsToDisk(meta.Tag); err != nil {
		JSON(w, r, 400, err.Error())
		return
	}

	tunnelMapRange(func(t *TUN) bool {
		m := t.meta.Load()
		if m == nil || m.Tag != form.Tag {
			return true
		}
		if t.GetState() >= TUN_Connected {
			if err := t.AnnounceAllowedHosts(hosts, form.AllowAll); err != nil {
				DEBUG("peer list announce failed: ", err)
			}
		}
		return false
	})

	JSON(w, r, 200, hosts)
}

func HTTP_SetConfig(w http.ResponseWriter, r *http.Request) {
	config := new(configV2)
	err := Bind(config, r)
	if err != nil {
		JSON(w, r, 400, err.Error())
		return
	}

	err = SetConfig(config)
	if err != nil {
		JSON(w, r, 400, err.Error())
		return
	}
	JSON(w, r, 200, nil)
}

func HTTP_GetQRCode(w http.ResponseWriter, r *http.Request) {
	form := new(TWO_FACTOR_CONFIRM)
	err := Bind(form, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	QR, err := GetQRCode(form)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, QR)
}

func HTTP_CreateTunnel(w http.ResponseWriter, r *http.Request) {
	tun, err := createRandomTunnel()
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, tun)
}

func HTTP_DeleteTunnels(w http.ResponseWriter, r *http.Request) {
	form := new(TunnelMETA)
	err := Bind(form, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	// Reject a traversing tag before it reaches os.Remove (arbitrary file delete).
	if !safeTunnelTag(form.Tag) {
		JSON(w, r, 400, "invalid tunnel tag")
		return
	}

	state := STATE.Load()
	ext := tunnelFileSuffix
	if storedTun, ok := TunnelMetaMap.Load(form.Tag); ok {
		if storedTun.ConfigFormat != "" {
			ext = storedTun.ConfigFormat
		}
	}
	_ = os.Remove(state.TunnelsPath + form.Tag + ext)

	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		if tun.Tag == form.Tag {
			TunnelMetaMap.Delete(form.Tag)
			return false
		}
		return true
	})

	JSON(w, r, 200, nil)
}

func HTTP_ForwardToController(w http.ResponseWriter, r *http.Request) {
	form := new(FORWARD_REQUEST)
	err := Bind(form, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	data, code := ForwardToController(form)
	JSON(w, r, code, data)
}

func HTTP_CreateDeviceWithKeys(w http.ResponseWriter, r *http.Request) {
	form := new(CreateDeviceWithKeysForm)
	if err := Bind(form, r); err != nil {
		JSON(w, r, 400, err)
		return
	}
	data, code := CreateDeviceWithKeys(form)
	JSON(w, r, code, data)
}
