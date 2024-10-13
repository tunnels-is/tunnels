package core

import (
	"crypto/tls"
	"io/fs"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	m "github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/websocket"
)

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(DIST_EMBED, "dist")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func StartAPI(MONITOR chan int) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		time.Sleep(2 * time.Second)
		MONITOR <- 2
	}()

	E := echo.New()

	E.Use(m.SecureWithConfig(m.DefaultSecureConfig))

	corsConfig := m.CORSConfig{
		Skipper:      m.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"POST", "GET", "OPTIONS"},
		AllowHeaders: []string{"*"},
	}

	E.Use(m.CORSWithConfig(corsConfig))

	if !NATIVE {
		assetHandler := http.FileServer(getFileSystem())
		E.GET("/", echo.WrapHandler(assetHandler))
		E.GET("/assets/*", echo.WrapHandler(assetHandler))
	}

	// E.GET("/state", WS_STATE)
	E.GET("/logs", WS_LOGS)

	v1 := E.Group("/v1")
	v1.POST("/method/:method", serveMethod)

	tlsConfig := new(tls.Config)
	tlsConfig.MinVersion = tls.VersionTLS13
	if C.APIAutoTLS && !NATIVE {

		if C.APICertIPs == nil || len(C.APICertIPs) < 1 {
			C.APICertIPs = []string{"127.0.0.1"}
		}

		if C.APICertDomains == nil || len(C.APICertDomains) < 1 {
			C.APICertDomains = []string{"tunnels.app", "app.tunnels.is"}
		}

		certs, err := MakeCert(C.APICertType, C.APICertIPs, C.APICertDomains)
		if err != nil {
			ERROR("Certificate error:", err)
			return
		}
		tlsConfig.Certificates = []tls.Certificate{certs}
	}

	API_SERVER = http.Server{
		Handler:   E,
		TLSConfig: tlsConfig,
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
	if NATIVE {
		port = "0"
	}

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

	if C.APICert != "" && C.APIKey != "" {
		if err := API_SERVER.ServeTLS(ln, C.APICert, C.APIKey); err != http.ErrServerClosed {
			ERROR("api start error: ", err)
		}
	} else if C.APIAutoTLS {
		if err := API_SERVER.ServeTLS(ln, "", ""); err != http.ErrServerClosed {
			ERROR("api start error: ", err)
		}
	} else {
		if err := API_SERVER.Serve(ln); err != http.ErrServerClosed {
			ERROR("api start error: ", err)
		}
	}
}

func serveMethod(e echo.Context) error {
	method := e.Param("method")
	switch method {
	case "connectPrivate":
		return HTTP_ConnectPrivate(e)
	case "connect":
		return HTTP_Connect(e)
	case "disconnect":
		return HTTP_Disconnect(e)
	case "resetNetwork":
		return HTTP_ResetNetwork(e)
	case "setConfig":
		return HTTP_SetConfig(e)
	case "getQRCode":
		return HTTP_GetQRCode(e)
	case "forwardToController":
		return HTTP_ForwardToController(e)
	case "createConnection":
		return HTTP_CreateConnection(e)

	case "getState":
		return HTTP_GetState(e)
	default:
	}
	return e.JSON(200, nil)
}

var LogSocket *websocket.Conn

func WS_LOGS(c echo.Context) error {
	websocket.Handler(func(ws *websocket.Conn) {
		defer func() {
			r := recover()
			if r != nil {
				DEBUG(r, string(debug.Stack()))
			}
			ws.Close()
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
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func HTTP_GetState(e echo.Context) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()
	form := new(FORWARD_REQUEST)
	err = e.Bind(form)
	if err != nil {
		return e.JSON(400, err)
	}

	_ = PrepareState()
	return e.JSON(200, GLOBAL_STATE)
}

func HTTP_ConnectPrivate(e echo.Context) (err error) {
	ns := new(ConnectionRequest)
	err = e.Bind(ns)
	if err != nil {
		return e.JSON(400, err)
	}

	// code, err := ConnectToPrivateNode(*ns)
	// if err != nil {
	// 	return e.String(code, err.Error())
	// }
	return e.JSON(0, nil)
}

func HTTP_Connect(e echo.Context) (err error) {
	ns := new(UIConnectRequest)
	err = e.Bind(ns)
	if err != nil {
		return e.JSON(400, err)
	}

	code, err := PublicConnect(*ns)
	if err != nil {
		return e.String(code, err.Error())
	}
	return e.JSON(code, nil)
}

func HTTP_Disconnect(e echo.Context) (err error) {
	DF := new(DisconnectForm)
	err = e.Bind(DF)
	if err != nil {
		return e.JSON(400, err)
	}
	err = Disconnect(DF.GUID, true, false)
	if err != nil {
		return e.JSON(400, err)
	}
	return e.JSON(200, nil)
}

func HTTP_ResetNetwork(e echo.Context) (err error) {
	ResetEverything()
	return e.JSON(200, nil)
}

func HTTP_SetConfig(e echo.Context) (err error) {
	config := new(Config)
	err = e.Bind(config)
	if err != nil {
		return e.JSON(400, err.Error())
	}

	err = SetConfig(config)
	if err != nil {
		return e.JSON(400, err.Error())
	}
	return e.JSON(200, nil)
}

func HTTP_GetQRCode(e echo.Context) (err error) {
	form := new(TWO_FACTOR_CONFIRM)
	err = e.Bind(form)
	if err != nil {
		return e.JSON(400, err)
	}
	QR, err := GetQRCode(form)
	if err != nil {
		return e.JSON(400, err)
	}
	return e.JSON(200, QR)
}

func HTTP_CreateConnection(e echo.Context) (err error) {
	config, err := createRandomTunnel()
	if err != nil {
		return e.String(400, err.Error())
	}
	return e.JSON(200, config)
}

func HTTP_ForwardToController(e echo.Context) (err error) {
	form := new(FORWARD_REQUEST)
	err = e.Bind(form)
	if err != nil {
		return e.JSON(400, err)
	}
	data, code := ForwardToController(form)
	return e.JSON(code, data)
}
