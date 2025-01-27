package main

import (
	"context"
	"time"

	service "github.com/tunnels-is/tunnels/cmd/service"
	"github.com/tunnels-is/tunnels/core"
)

var monitor = make(chan int, 100)

func main() {
	ctx := context.Background()

	core.MINIMAL = true
	core.BASE_PATH = "./"

	core.CLIHostname = "your-custom-hostname"
	core.CLIDNS = "vpn.your-domain.com"
	core.CLIDeviceKey = "your-device-key"

	go service.StartWithExternalMonitor(ctx, 1, monitor)
	for {
		select {
		case id := <-monitor:
			go service.StartWithExternalMonitor(ctx, id, monitor)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}
