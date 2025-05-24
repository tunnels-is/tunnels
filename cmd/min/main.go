package main

import (
	"flag"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	cli := core.CLI.Load()
	flag.StringVar(&cli.AuthServer, "auth", "api.tunnels.is", "The auth server you want to use")
	flag.StringVar(&cli.DeviceToken, "device", "", "the device token")
	flag.StringVar(&cli.DeviceToken, "serverID", "", "the server you want to connect to")
	cli.Minimal = true
	core.CLI.Store(cli)
	flag.Parse()

	// CAN WE SKIP DISK CONFIG ??

	// GET SERVER AND DEVICE
	// create default tunnel + assign
	// connect ?

	service.Start()
}
