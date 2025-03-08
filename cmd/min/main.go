package main

import (
	"flag"
	"fmt"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	cli := core.CLI.Load()
	flag.StringVar(&cli.DNS, "DNS", "", "Tunnels will use this DNS to fetch connection info ( optional )")

	// Non-DNS startup options
	flag.StringVar(&cli.Host, "host", "", "Custom server hostname ( not needed if dns is used )")
	flag.StringVar(&cli.Port, "port", "", "Custom port ( not needed if dns is used )")
	flag.StringVar(&cli.ServerID, "serverID", "", "Server ID ( not needed if dns is used )")

	// Other
	flag.StringVar(&cli.Hostname, "hostname", "", "Custom hostname for this device")
	flag.StringVar(&cli.DeviceKey, "deviceKey", "", "Device Key used to authenticate your account")
	flag.BoolVar(&cli.DisableVPLFirewall, "disableVPLFirewall", false, "Disable the VPL firewall, allowing all devices to communicate with this device")
	flag.StringVar(&cli.BasePath, "basePath", "", "manualy set base path for the config and log files ( optional, default location is in the binary dir )")
	flag.Parse()

	if cli.DeviceKey == "" {
		fmt.Println("-deviceKey missing")
		return
	}

	core.POPUI = false
	core.MINIMAL = true
	service.Start()
}
