package main

import (
	"flag"

	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	// flag.StringVar(&core.CLIOrgId, "orgID", "", "Organization ID (skip this for user API Token connections)")
	// flag.StringVar(&core.CLIDeviceKey, "deviceKey", "", "DeviceKey or API Token")
	flag.StringVar(&core.BASE_PATH, "basePath", "", "manualy set base path for the config and log files")
	flag.Parse()

	// if core.CLIDeviceKey == "" {
	// 	fmt.Println("--deviceKey missing")
	// }

	if core.BASE_PATH == "" {
		core.BASE_PATH = "."
	}

	core.NATIVE = false
	service.StartMinimal()
}
