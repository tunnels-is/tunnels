package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"runtime"
	rdebug "runtime/debug"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed dist
var DIST embed.FS

//go:embed wintun.dll
var DLL embed.FS

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	s := client.STATE.Load()
	flag.StringVar(&s.BasePath, "basePath", "", "manually set base path for config and log files")
	flag.StringVar(&s.TunnelType, "tunnelType", "default", "tunnel type: default, strict, iot")
	flag.BoolVar(&s.Debug, "debug", false, "enable debug logging")
	flag.BoolVar(&s.RequireConfig, "requireConfig", false, "require config file to start")
	flag.Parse()
	client.STATE.Store(s)

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	client.DIST_EMBED = DIST
	client.DLL_EMBED = DLL

	// Initialize the VPN client synchronously so config is available
	// before building the Wails app.
	runtime.GOMAXPROCS(runtime.NumCPU())
	if err := client.InitService(); err != nil {
		log.Fatal("Failed to initialize tunnels: ", err)
	}

	// Disable browser auto-open since Wails provides the window.
	conf := client.CONFIG.Load()
	conf.OpenUI = false
	client.CONFIG.Store(conf)

	// Resolve the API server address from the loaded config.
	apiIP := conf.APIIP
	if apiIP == "" {
		apiIP = client.DefaultAPIIP
	}
	apiPort := conf.APIPort
	if apiPort == "" {
		apiPort = client.DefaultAPIPort
	}
	apiAddr := apiIP + ":" + apiPort
	backendURL := "http://" + apiAddr

	// Start the VPN client event loop in the background.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r, string(rdebug.Stack()))
			}
		}()
		client.LaunchTunnels()
	}()

	// Wait for the API server to accept connections.
	waitForAPI(apiAddr)

	handler := newTunnelsHandler(backendURL)

	if err := wails.Run(&options.App{
		Title:            "Tunnels",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		BackgroundColour:         options.NewRGB(24, 24, 27),
		EnableDefaultContextMenu: true,
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		OnShutdown: func(ctx context.Context) {
			if client.CancelFunc != nil {
				client.CancelFunc()
			}
			client.ResetEverything()
		},
	}); err != nil {
		log.Fatal(err)
	}
}

// tunnelsHandler proxies all requests to the client's HTTPS API server.
// For SPA routes (paths without file extensions that aren't API calls),
// it serves index.html with an injected script that replaces WebSocket
// log streaming with HTTP polling through the Wails proxy.
type tunnelsHandler struct {
	proxy      *httputil.ReverseProxy
	backendURL string
	httpClient *http.Client
}

func newTunnelsHandler(backendURL string) *tunnelsHandler {
	target, _ := url.Parse(backendURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	return &tunnelsHandler{
		proxy:      proxy,
		backendURL: backendURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (h *tunnelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	// Non-GET requests (API POST calls) go straight to the proxy.
	if r.Method != http.MethodGet {
		h.proxy.ServeHTTP(w, r)
		return
	}

	// API endpoints and static asset paths go to the proxy.
	if strings.HasPrefix(p, "/v1/") || strings.HasPrefix(p, "/assets/") {
		h.proxy.ServeHTTP(w, r)
		return
	}

	// Requests for files with extensions (favicon.ico, etc.) go to the proxy.
	if path.Ext(p) != "" {
		h.proxy.ServeHTTP(w, r)
		return
	}

	// All other GET requests are SPA routes — serve index.html
	// with the WebSocket override injected.
	h.serveIndex(w, r)
}

// serveIndex serves index.html from the backend for SPA route fallback.
// The frontend detects Wails and directs API/WebSocket calls to the
// backend itself, so no script injection is needed here.
func (h *tunnelsHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	resp, err := h.httpClient.Get(h.backendURL + "/")
	if err != nil {
		h.proxy.ServeHTTP(w, r)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// waitForAPI polls until the client's API server is accepting TCP connections.
func waitForAPI(addr string) {
	for {
		conn, err := net.DialTimeout("tcp4", addr, time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
