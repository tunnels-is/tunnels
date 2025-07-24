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

//go:embed dist
var DIST embed.FS

//go:embed wintun.dll
var DLL embed.FS

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	s := client.STATE.Load()
	createConfig := flag.Bool("createConfig", false, "generate a default config and exit")
	flag.StringVar(&s.BasePath, "basePath", "", "manualy set base path for the config and log files")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(1)
	}

	client.CreateConfig(createConfig)

	client.DIST_EMBED = DIST
	client.DLL_EMBED = DLL
	service.Start(false)
}
