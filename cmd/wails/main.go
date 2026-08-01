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
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed dist
var DIST embed.FS

//go:embed wintun.dll
var DLL embed.FS

// App icon (PNG) for Linux window / macOS About.
// Windows title-bar + taskbar icons come from rsrc_windows_*.syso (icon resource ID 3 = Wails AppIconID).
//
//go:embed build/appicon.png
var appIcon []byte

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

	runtime.GOMAXPROCS(runtime.NumCPU())
	if err := client.InitService(); err != nil {
		log.Fatal("Failed to initialize tunnels: ", err)
	}

	conf := client.CONFIG.Load()
	conf.OpenUI = false
	client.CONFIG.Store(conf)

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

	dialAddr := net.JoinHostPort(uiHost, apiPort)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r, string(rdebug.Stack()))
			}
		}()
		client.LaunchTunnels()
	}()

	waitForAPI(dialAddr)

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
		Windows: &windows.Options{
			DisableWindowIcon:    false,
			IsZoomControlEnabled: true, // Ctrl+mouse wheel / keyboard zoom in WebView2
			DisablePinchZoom:     false,
		},
		Linux: &linux.Options{
			Icon: appIcon,
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title: "Tunnels",
				Icon:  appIcon,
			},
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
