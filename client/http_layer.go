package client

import (
	"crypto/tls"
	"encoding/json"
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
		var req ConnectionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		code, err := ConnectInternal(&req)
		if err != nil {
			STRING(w, r, code, err.Error())
			return
		}
		JSON(w, r, code, nil)
	case "disconnect":
		var form DisconnectForm
		_ = json.NewDecoder(r.Body).Decode(&form)
		err := DisconnectInternal(&form)
		if err != nil {
			JSON(w, r, 400, err.Error())
			return
		}
		JSON(w, r, 200, nil)
	case "resetNetwork":
		ResetNetworkInternal()
		JSON(w, r, 200, nil)
	case "getQRCode":
		var form TWO_FACTOR_CONFIRM
		_ = json.NewDecoder(r.Body).Decode(&form)
		QR, err := GetQRCodeInternal(&form)
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, QR)
	case "forwardToController":
		var form FORWARD_REQUEST
		_ = json.NewDecoder(r.Body).Decode(&form)
		data, code := ForwardToControllerInternal(&form)
		JSON(w, r, code, data)
	case "createTunnel":
		tun, err := CreateTunnelInternal()
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, tun)
	case "deleteTunnel":
		var form TunnelMETA
		_ = json.NewDecoder(r.Body).Decode(&form)
		err := DeleteTunnelsInternal(&form)
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, nil)
	case "setUser":
		var u User
		_ = json.NewDecoder(r.Body).Decode(&u)
		err := SetUserInternal(&u)
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, nil)
	case "getUser":
		u, err := GetUserInternal()
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, u)
	case "delUser":
		err := DelUserInternal()
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, nil)
	case "getState":
		JSON(w, r, 200, GetStateInternal())
	case "setConfig":
		var config configV2
		_ = json.NewDecoder(r.Body).Decode(&config)
		err := SetConfigInternal(&config)
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, nil)
	case "setTunnel":
		var newForm saveTunnelForm
		_ = json.NewDecoder(r.Body).Decode(&newForm)
		err := SetTunnelInternal(&newForm)
		if err != nil {
			JSON(w, r, 400, err)
			return
		}
		JSON(w, r, 200, nil)
	case "getDNSStats":
		JSON(w, r, 200, GetDNSStatsInternal())
	default:
		w.WriteHeader(200)
		r.Body.Close()
	}
}

// --- Refactored internal logic functions ---

func ConnectInternal(req *ConnectionRequest) (int, error) {
	return PublicConnect(req)
}

func DisconnectInternal(form *DisconnectForm) error {
	return Disconnect(form.ID, true)
}

func ResetNetworkInternal() {
	ResetEverything()
}

func GetQRCodeInternal(form *TWO_FACTOR_CONFIRM) (any, error) {
	return GetQRCode(form)
}

func ForwardToControllerInternal(form *FORWARD_REQUEST) (any, int) {
	return ForwardToController(form)
}

func CreateTunnelInternal() (any, error) {
	return createRandomTunnel()
}

func DeleteTunnelsInternal(form *TunnelMETA) error {
	state := STATE.Load()
	_ = os.Remove(state.TunnelsPath + form.Tag + tunnelFileSuffix)
	// Remove from map
	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		if tun.Tag == form.Tag {
			TunnelMetaMap.Delete(form.Tag)
			return false
		}
		return true
	})
	return nil
}

func SetUserInternal(u *User) error {
	if err := saveUser(u); err != nil {
		return err
	}
	return nil
}

func GetUserInternal() (*User, error) {
	return loadUser()
}

func DelUserInternal() error {
	return delUser()
}

func GetStateInternal() *StateResponse {
	return GetFullState()
}

func SetConfigInternal(config *configV2) error {
	return SetConfig(config)
}

func SetTunnelInternal(newForm *saveTunnelForm) error {
	isConnected := false
	tunnelMapRange(func(t *TUN) bool {
		if t.CR != nil && t.CR.Tag == newForm.Meta.Tag {
			isConnected = true
			return false
		}
		return true
	})
	if isConnected {
		return &CustomError{"tunnel is connected"}
	}
	errs := validateTunnelMeta(newForm.Meta, newForm.OldTag)
	if len(errs) > 0 {
		return &CustomError{"validation error"}
	}
	TunnelMetaMap.Store(newForm.Meta.Tag, newForm.Meta)
	if err := writeTunnelsToDisk(newForm.Meta.Tag); err != nil {
		return err
	}
	if newForm.OldTag != newForm.Meta.Tag {
		TunnelMetaMap.Delete(newForm.OldTag)
		state := STATE.Load()
		if err := os.Remove(state.TunnelsPath + newForm.OldTag + tunnelFileSuffix); err != nil {
			return err
		}
	}
	return nil
}

func GetTunnelsInternal() []*TunnelMETA {
	out := make([]*TunnelMETA, 0)
	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		out = append(out, tun)
		return true
	})
	return out
}

func GetDNSStatsInternal() map[string]any {
	stats := make(map[string]any)
	DNSStatsMap.Range(func(key, value any) bool {
		ks, ok := key.(string)
		if !ok {
			return true
		}
		stats[ks] = value
		return true
	})
	return stats
}

type CustomError struct {
	Msg string
}

func (e *CustomError) Error() string {
	return e.Msg
}

// APIBridge provides Wails JS bindings for HTTPHandler logic
// Wails will generate JS bindings for exported methods on this struct
// You can extend this struct with more methods as needed
//go:generate wails generate go

type APIBridge struct{}

// CallMethod exposes the HTTPHandler logic to Wails JS
// method: the API method (e.g., "connect", "getState", etc.)
// payload: the request body as JSON string
// Returns: response as JSON string, or error string
func (a *APIBridge) CallMethod(method string, payload string) (string, error) {
	switch method {
	case "connect":
		var req ConnectionRequest
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			return "", err
		}
		code, err := ConnectInternal(&req)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error(), "code": code})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"code": code})
		return string(b), nil
	case "disconnect":
		var form DisconnectForm
		if err := json.Unmarshal([]byte(payload), &form); err != nil {
			return "", err
		}
		err := DisconnectInternal(&form)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "resetNetwork":
		ResetNetworkInternal()
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "getQRCode":
		var form TWO_FACTOR_CONFIRM
		if err := json.Unmarshal([]byte(payload), &form); err != nil {
			return "", err
		}
		QR, err := GetQRCodeInternal(&form)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(QR)
		return string(b), nil
	case "forwardToController":
		var form FORWARD_REQUEST
		if err := json.Unmarshal([]byte(payload), &form); err != nil {
			return "", err
		}
		data, code := ForwardToControllerInternal(&form)
		b, _ := json.Marshal(map[string]any{"code": code, "data": data})
		return string(b), nil
	case "createTunnel":
		tun, err := CreateTunnelInternal()
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(tun)
		return string(b), nil
	case "deleteTunnel":
		var form TunnelMETA
		if err := json.Unmarshal([]byte(payload), &form); err != nil {
			return "", err
		}
		err := DeleteTunnelsInternal(&form)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "setUser":
		var u User
		if err := json.Unmarshal([]byte(payload), &u); err != nil {
			return "", err
		}
		err := SetUserInternal(&u)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "getUser":
		u, err := GetUserInternal()
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(u)
		return string(b), nil
	case "delUser":
		err := DelUserInternal()
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "getState":
		b, _ := json.Marshal(GetStateInternal())
		return string(b), nil
	case "setConfig":
		var config configV2
		if err := json.Unmarshal([]byte(payload), &config); err != nil {
			return "", err
		}
		err := SetConfigInternal(&config)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "setTunnel":
		var newForm saveTunnelForm
		if err := json.Unmarshal([]byte(payload), &newForm); err != nil {
			return "", err
		}
		err := SetTunnelInternal(&newForm)
		if err != nil {
			b, _ := json.Marshal(map[string]any{"error": err.Error()})
			return string(b), nil
		}
		b, _ := json.Marshal(map[string]any{"success": true})
		return string(b), nil
	case "getDNSStats":
		b, _ := json.Marshal(GetDNSStatsInternal())
		return string(b), nil
	default:
		return "", nil
	}
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
	defer RecoverAndLogToFile()
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
