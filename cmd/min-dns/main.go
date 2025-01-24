package main

import (
	"github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	core.CLIHostname = "custom-hostname"
	core.CLIDNS = "cert-test.tunnels.is"
	core.CLIDeviceKey = ""
	core.CLIDisableBlockLists = true
	core.CLIDisableVPLFirewall = true

	core.BASE_PATH = "./"
	core.MINIMAL = true
	core.NATIVE = false

	core.NATIVE = false
	core.MINIMAL = true
	service.Start()
}
