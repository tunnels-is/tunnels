package core

import (
	"crypto/tls"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"golang.org/x/net/websocket"
)

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(DIST_EMBED, "dist")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func makeTLSConfig() (tc *tls.Config) {
	tc = new(tls.Config)
	tc.MinVersion = tls.VersionTLS13
	certsExist := true
	_, err := os.Stat(C.APICert)
	if err != nil {
		certsExist = false
	}
	_, err = os.Stat(C.APIKey)
	if err != nil {
		certsExist = false
	}

	if !certsExist {

		_, err := certs.MakeCert(
			C.APICertType,
			C.APICert,
			C.APIKey,
			C.APICertIPs,
			C.APICertDomains,
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

func LaunchAPI(MONITOR chan int) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		time.Sleep(2 * time.Second)
		MONITOR <- 2
	}()
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

	ip := C.APIIP
	if ip == "" {
		GLOBAL_STATE.C.APIIP = "127.0.0.1"
		ip = "127.0.0.1"
	}

	port := C.APIPort
	if port == "" {
		GLOBAL_STATE.C.APIPort = "7777"
		port = "7777"
	}

	// if we are running wails, we want
	// the computer to select a port for us.

	API_SERVER.Addr = ip + ":" + port
	ln, err := net.Listen("tcp4", API_SERVER.Addr)
	if err != nil {
		ERROR("api start error: ", err)
		return
	}

	addr := strings.Split(ln.Addr().String(), ":")
	API_PORT = addr[len(addr)-1]
	C.APIPort = API_PORT
	if C.APIIP != "0.0.0.0" {
		C.APIIP = addr[0]
	}
	INFO("====== API SERVER =========")
	INFO("ADDR: ", ln.Addr())
	INFO("PORT: ", C.APIPort)
	INFO("IP: ", C.APIIP)
	INFO("Key: ", C.APIKey)
	INFO("Cert: ", C.APICert)
	INFO("===========================")

	select {
	case uiChan <- struct{}{}:
	default:
	}

	if err := API_SERVER.ServeTLS(ln, C.APICert, C.APIKey); err != http.ErrServerClosed {
		ERROR("api start error: ", err)
	}
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
	case "setConfig":
		HTTP_SetConfig(w, r)
		return
	case "getQRCode":
		HTTP_GetQRCode(w, r)
		return
	case "forwardToController":
		HTTP_ForwardToController(w, r)
		return
	case "createConnection":
		HTTP_CreateConnection(w, r)
		return
	case "getState":
		HTTP_GetState(w, r)
		return
	default:
	}
	w.WriteHeader(200)
	r.Body.Close()
	return
}

var LogSocket *websocket.Conn

func handleWebSocket(ws *websocket.Conn) {
	defer RecoverAndLogToFile()
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()
	LogSocket = ws
	for {
		select {
		case event := <-APILogQueue:
			err := websocket.Message.Send(ws, event)
			if err != nil {
				// Make an attempt to delive this log line to the new LogSocket.
				// if delivery fails, the event will be found in the log file.
				_ = websocket.Message.Send(LogSocket, event)
				ERROR("Logging websocket error: ", err)
				return
			}
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
	w.Write([]byte(data))
}

func JSON(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
	defer RecoverAndLogToFile()
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()
	w.WriteHeader(code)
	encoder := json.NewEncoder(w)
	err := encoder.Encode(data)
	if err != nil {
		ERROR("Unable to write encoded json to response writer:", err)
		return
	}
}

func HTTP_GetState(w http.ResponseWriter, r *http.Request) {
	form := new(FORWARD_REQUEST)
	err := Bind(form, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	_ = GenerateState()
	JSON(w, r, 200, GLOBAL_STATE)
}

func HTTP_Connect(w http.ResponseWriter, r *http.Request) {
	ns := new(ConnectionRequest)
	err := Bind(ns, r)
	if err != nil {
		JSON(w, r, 400, err)
		return
	}

	code, err := PublicConnect(*ns)
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
	err = Disconnect(DF.GUID, true, false)
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

func HTTP_SetConfig(w http.ResponseWriter, r *http.Request) {
	config := new(Config)
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

func HTTP_CreateConnection(w http.ResponseWriter, r *http.Request) {
	config, err := createRandomTunnel()
	if err != nil {
		STRING(w, r, 400, err.Error())
		return
	}
	JSON(w, r, 200, config)
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
