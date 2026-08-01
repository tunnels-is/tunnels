package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	rdebug "runtime/debug"
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

	// Resolve the API bind address and the URL the webview should load.
	// The UI always targets loopback even when the API binds on 0.0.0.0.
	apiIP := conf.APIIP
	if apiIP == "" {
		apiIP = client.DefaultAPIIP
	}
	apiPort := conf.APIPort
	if apiPort == "" {
		apiPort = client.DefaultAPIPort
	}
	uiHost := apiIP
	if ip := net.ParseIP(apiIP); ip != nil && ip.IsUnspecified() {
		uiHost = "127.0.0.1"
	}
	uiURL := "http://" + net.JoinHostPort(uiHost, apiPort)
	// Dial target for readiness: can't dial 0.0.0.0 usefully on all platforms.
	dialAddr := net.JoinHostPort(uiHost, apiPort)

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
	waitForAPI(dialAddr)

	// Wails always boots at wails.localhost; redirect once into the local API
	// so the webview is same-origin with /v1 and the session cookie.
	if err := wails.Run(&options.App{
		Title:                    "Tunnels",
		Width:                    1280,
		Height:                   800,
		MinWidth:                 800,
		MinHeight:                600,
		BackgroundColour:         options.NewRGB(24, 24, 27),
		EnableDefaultContextMenu: true,
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, uiURL+r.URL.RequestURI(), http.StatusFound)
			}),
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
