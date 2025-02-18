package main

import (
	"embed"
	"flag"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

//go:embed dist
var DIST embed.FS

//go:embed wintun.dll
var DLL embed.FS

func main() {
	createConfig := flag.Bool("createConfig", false, "generate a default config and exit")
	flag.StringVar(&core.BASE_PATH, "basePath", "", "manualy set base path for the config and log files")
	flag.Parse()

	core.CreateConfig(createConfig)

	core.DIST_EMBED = DIST
	core.DLL_EMBED = DLL
	core.POPUI = true
	service.Start()
}
