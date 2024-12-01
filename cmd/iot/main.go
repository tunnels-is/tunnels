package main

import (
	"flag"
	"fmt"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

// vpn.domain.com
// ip:port:cert_sub_domain
// cert_sub_domain.vpn.domain.com

// CONST: orgID - ??
// CONST: DeviceKey - ??

func main() {
	flag.StringVar(&core.CLIOrgId, "DNS", "", "Tunnels will use this DNS to fetch connection info ( optional )")

	flag.StringVar(&core.CLIOrgId, "orgID", "", "Organization ID (only use if DNS is enabled)")
	flag.StringVar(&core.CLIDeviceKey, "deviceKey", "", "Device Key (only use if DNS is enabled)")
	flag.StringVar(&core.CLIHostname, "hostname", "", "Custom hostname for this device")

	flag.StringVar(&core.BASE_PATH, "basePath", "", "manualy set base path for the config and log files")
	flag.Parse()

	if core.CLIOrgId == "" {
		fmt.Println("--orgID missing")
	}

	// Config can be applied manually
	// core.GLOBAL_STATE.C = new(core.Config)
	// core.GLOBAL_STATE.C.APIPort = "445"
	// etc...

	if core.CLIDeviceKey == "" {
		fmt.Println("--deviceKey missing")
	}

	if core.BASE_PATH == "" {
		core.BASE_PATH = "."
	}

	core.NATIVE = false
	core.IOT = true
	service.Start()
}
