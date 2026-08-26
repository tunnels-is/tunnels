package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	rdebug "runtime/debug"

	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/ui"
	"github.com/tunnels-is/tunnels/version"
)

//go:embed wintun.dll
var DLL embed.FS

//go:embed appicon.png
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

	client.DLL_EMBED = DLL

	runtime.GOMAXPROCS(runtime.NumCPU())
	if err := client.InitService(); err != nil {
		log.Fatal("Failed to initialize tunnels: ", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r, string(rdebug.Stack()))
			}
		}()
		client.LaunchTunnels()
	}()

	ui.Run(appIcon)
}
