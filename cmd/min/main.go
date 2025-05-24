package main

import (
	"flag"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	cli := core.CLIConfig.Load()
	flag.StringVar(&cli.AuthServer, "authHost", "api.tunnels.is", "The auth server you want to use")
	flag.StringVar(&cli.DeviceID, "deviceID", "", "the device token")
	flag.StringVar(&cli.ServerID, "serverID", "", "the server you want to connect to")
	flag.BoolVar(&cli.DNS, "dns", false, "enable dns server")
	flag.BoolVar(&cli.Secure, "secure", false, "validate TLS certificate")
	flag.BoolVar(&cli.SendStats, "sendStats", true, "send device statistics")
	cli.Enabled = true
	core.CLIConfig.Store(cli)
	flag.Parse()

	service.Start(true)
}
