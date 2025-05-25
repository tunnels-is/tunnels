package main

import (
	"embed"
	"flag"

	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/cmd/service"
)

//go:embed dist
var DIST embed.FS

//go:embed wintun.dll
var DLL embed.FS

func main() {
	s := client.STATE.Load()
	createConfig := flag.Bool("createConfig", false, "generate a default config and exit")
	flag.StringVar(&s.BasePath, "basePath", "", "manualy set base path for the config and log files")
	flag.Parse()

	client.CreateConfig(createConfig)

	client.DIST_EMBED = DIST
	client.DLL_EMBED = DLL
	service.Start(false)
}
