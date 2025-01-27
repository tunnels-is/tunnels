package main

import (
	"flag"
	"fmt"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	flag.StringVar(&core.CLIDNS, "DNS", "", "Tunnels will use this DNS to fetch connection info ( optional )")

	// Non-DNS startup options
	flag.StringVar(&core.CLIHost, "host", "", "Custom server hostname ( not needed if dns is used )")
	flag.StringVar(&core.CLIPort, "port", "", "Custom port ( not needed if dns is used )")
	flag.StringVar(&core.CLIServerID, "serverID", "", "Server ID ( not needed if dns is used )")

	// Other
	flag.StringVar(&core.CLIHostname, "hostname", "", "Custom hostname for this device")
	flag.StringVar(&core.CLIDeviceKey, "deviceKey", "", "Device Key used to authenticate your account")
	flag.BoolVar(&core.CLIDisableVPLFirewall, "disableVPLFirewall", false, "Disable the VPL firewall, allowing all devices to communicate with this device")
	flag.StringVar(&core.BASE_PATH, "basePath", "", "manualy set base path for the config and log files ( optional, default location is in the binary dir )")
	flag.Parse()

	if core.CLIDeviceKey == "" {
		fmt.Println("--deviceKey missing")
	}

	if core.BASE_PATH == "" {
		core.BASE_PATH = "."
	}

	core.POPUI = false
	core.MINIMAL = true
	service.Start()
}
