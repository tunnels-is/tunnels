package main

import (
	"flag"

	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/cmd/service"
)

func main() {
	cli := client.CLIConfig.Load()
	flag.StringVar(&cli.AuthServer, "authHost", "api.tunnels.is", "The auth server you want to use")
	flag.StringVar(&cli.DeviceID, "deviceID", "", "the device token")
	flag.StringVar(&cli.ServerID, "serverID", "", "the server you want to connect to")
	// flag.BoolVar(&cli.DNS, "dns", false, "enable dns server")
	flag.BoolVar(&cli.Secure, "secure", false, "validate TLS certificate")
	flag.BoolVar(&cli.SendStats, "sendStats", true, "send device statistics")
	flag.StringVar(&cli.Hostname, "hostname", "", "device hostname")
	cli.Enabled = true
	client.CLIConfig.Store(cli)
	flag.Parse()

	// device, err := client.GetDeviceByID(cli.Secure, cli.AuthServer, cli.DeviceID)
	// if err != nil {
	// 	panic(err)
	// }
	// cli.DeviceID = string(device.ID)

	service.Start(true)
}
