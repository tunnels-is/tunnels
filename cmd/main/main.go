package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/version"
)

//go:embed wintun.dll
var DLL embed.FS

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	s := client.STATE.Load()
	flag.StringVar(&s.BasePath, "basePath", "", "manually set base path for the config and log files")
	flag.StringVar(&s.TunnelType, "tunnelType", "default", "defines which tunnel type should be automatically generate if no default tunnel/tunnels.conf exists. Available types: default, strict, iot")
	flag.BoolVar(&s.Debug, "debug", false, "manually enable debug")
	flag.BoolVar(&s.RequireConfig, "requireConfig", false, "Force tunnels to require disk config to start")
	flag.BoolVar(&client.EnableTLS, "tls", false, "enable TLS for the local API server")
	flag.BoolVar(&client.DevMode, "dev", false, "disable local API auth (for debugging only)")
	flag.BoolVar(&client.EnablePprof, "pprof", false, "enable net/http/pprof on the local API server (/debug/pprof/)")
	flag.Parse()
	client.STATE.Store(s)

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	client.DLL_EMBED = DLL
	service.Start()
}
