package core

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"golang.org/x/net/websocket"
)

func LaunchAPI() {
	defer RecoverAndLogToFile()

	assetHandler := http.FileServer(getFileSystem())

	mux := http.NewServeMux()
	mux.Handle("/logs", websocket.Handler(handleWebSocket))
	mux.Handle("/", assetHandler)
	mux.Handle("/assets/", assetHandler)
	mux.HandleFunc("/v1/method/{method}", HTTPhandler)
	API_SERVER = http.Server{
		Handler:   mux,
		TLSConfig: makeTLSConfig(),
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

	// addr := strings.Split(ln.Addr().String(), ":")
	// API_PORT = addr[len(addr)-1]
	// C.APIPort = API_PORT
	// if C.APIIP != "0.0.0.0" {
	// 	C.APIIP = addr[0]
	// }

	INFO("====== API SERVER =========")
	INFO("ADDR: ", ln.Addr())
	INFO("IP: ", ip)
	INFO("PORT: ", port)
	INFO("Key: ", conf.APIKey)
	INFO("Cert: ", conf.APICert)
	INFO("===========================")

	select {
	case uiChan <- struct{}{}:
	default:
	}

	if err := API_SERVER.ServeTLS(ln, conf.APICert, conf.APIKey); err != http.ErrServerClosed {
		ERROR("api start error: ", err)
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
	// tc.MinVersion = tls.VersionTLS12
	// tc.MaxVersion = tls.VersionTLS13
	// tc.CipherSuites = []uint16{
	// 	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	// 	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	// 	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	// 	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	// }
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

func setupCORS(w *http.ResponseWriter, _ *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}

func HTTPhandler(w http.ResponseWriter, r *http.Request) {
	setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		w.WriteHeader(204)
		r.Body.Close()
		return
	}

	method := r.PathValue("method")
	switch method {
	case "connect":
		HTTP_Connect(w, r)
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
	case "getUser":
		HTTP_GetUser(w, r)
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
	case "getDNSStats":
		HTTP_GetDNSStats(w, r)
		return
	default:
	}

	w.WriteHeader(200)
	r.Body.Close()
}

var LogSocket *websocket.Conn

func handleWebSocket(ws *websocket.Conn) {
	defer func() {
		if r := recover(); r != nil {
			ERROR("Possible UI reload: ", r, string(debug.Stack()))
		}
		if ws != nil {
			ws.Close()
		}
	}()

	LogSocket = ws
	for event := range APILogQueue {
		err := websocket.Message.Send(ws, event)
		if err != nil {
			// Make an attempt to delive this log line to the new LogSocket.
			// if delivery fails, the event will be found in the log file.
			_ = websocket.Message.Send(LogSocket, event)
			return
		}
	}

}

func Bind[I any](form I, r *http.Request) (err error) {
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
		w.WriteHeader(200)
		return
	}
	defer RecoverAndLogToFile()
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
	Config        *configV2
	State         *stateV2
	Tunnels       []*TunnelMETA
	ActiveTunnels []*TUN
	Network       StateNetworkResponse
}

type StateNetworkResponse struct {
	DefaultGateway       net.IP
	DefaultInterface     net.IP
	DefaultInterfaceID   int32
	DefaultInterfaceName string
}

func HTTP_GetDNSStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]any)
	DNSStatsMap.Range(func(key, value any) bool {
		ks, ok := key.(string)
		if !ok {
			return true
		}
		stats[ks] = value
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
	JSON(w, r, 200, nil)
}

func HTTP_DelUser(w http.ResponseWriter, r *http.Request) {
	JSON(w, r, 200, delUser())
}

func HTTP_GetUser(w http.ResponseWriter, r *http.Request) {
	u, err := loadUser()
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	JSON(w, r, 200, u)
}

func GetFullState() (s *StateResponse) {
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println(string(debug.Stack()))
		}
	}()
	state := STATE.Load()
	s = new(StateResponse)
	s.Version = version
	s.APIVersion = apiVersion
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

func HTTP_Disconnect(w http.ResponseWriter, r *http.Request) {
	DF := new(DisconnectForm)
	err := Bind(DF, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}
	err = Disconnect(DF.ID, true)
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

func HTTP_GetConfig(w http.ResponseWriter, r *http.Request) {
	JSON(w, r, 200, CONFIG.Load())
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

	// oldTun, ok := TunnelMetaMap.Load(newForm.OldTag)
	// if ok {
	// 	oldt, ok := oldTun.(*TunnelMETA)
	// 	if ok {
	// 		if !slices.Equal(oldt.AllowedHosts, newForm.Meta.AllowedHosts) {
	// 			tunnelMapRange(func(tun *TUN) bool {
	// 				meta := tun.meta.Load()
	// 				if meta.Tag == newForm.OldTag {

	// 					tc := &tls.Config{
	// 						MinVersion:         tls.VersionTLS13,
	// 						CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
	// 						InsecureSkipVerify: false,
	// 					}
	// 					var errm error
	// 					tc.RootCAs, errm = tun.LoadCertPEMBytes([]byte(tun.CR.ServerPubKey))
	// 					if errm != nil {
	// 						ERROR("Unable to load cert pem from controller: ", errm)
	// 					}

	// 					FR := &FirewallRequest{
	// 						DHCPToken:       tun.dhcp.Token,
	// 						IP:              net.IP(tun.dhcp.IP[:]).String(),
	// 						Hosts:           meta.AllowedHosts,
	// 						DisableFirewall: meta.DisableFirewall,
	// 					}

	// 					_, code, err := SendRequestToURL(
	// 						tc,
	// 						"POST",
	// 						"https://"+tun.CR.ServerIP+":"+tun.CR.ServerPort+"/v3/firewall",
	// 						FR,
	// 						10000,
	// 						tun.CR.Secure,
	// 					)
	// 					if err != nil {
	// 						ERROR("unable to update firewall: ", err)
	// 					} else if code != 200 {
	// 						ERROR("unable to update firewall: ", code)
	// 					} else {
	// 						DEBUG("firewall update on server")
	// 					}
	// 					return false
	// 				}
	// 				return true
	// 			})
	// 		}
	// 	}
	// }

	if newForm.OldTag != newForm.Meta.Tag {
		TunnelMetaMap.Delete(newForm.OldTag)
		state := STATE.Load()
		err = os.Remove(state.TunnelsPath + newForm.OldTag + tunnelFileSuffix)
		if err != nil {
			JSON(w, r, 400, err.Error())
			return
		}
	}

	JSON(w, r, 200, nil)
}

func HTTP_GetTunnels(w http.ResponseWriter, r *http.Request) {
	out := make([]*TunnelMETA, 0)
	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		out = append(out, tun)
		return true
	})

	JSON(w, r, 200, out)
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
	}

	state := STATE.Load()
	_ = os.Remove(state.TunnelsPath + form.Tag + tunnelFileSuffix)

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
	}
	data, code := ForwardToController(form)
	JSON(w, r, code, data)
}
