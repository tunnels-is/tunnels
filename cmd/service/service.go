package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/client"
)

func Start() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, string(debug.Stack()))
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	err := client.InitService()
	if err != nil {
		time.Sleep(5 * time.Second)
		fmt.Println("Unable to initialize tunnels:", err)
		os.Exit(1)
	}
	client.LaunchTunnels()
}

// StartWithExternalMonitor
// always start this as a goroutine
func StartWithExternalMonitor(ctx context.Context, minimal bool, id int, monitor chan int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			client.ERROR(r, string(debug.Stack()))
		}
		if ctx.Err() != nil {
			select {
			case monitor <- id:
			default:
			}
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	err = client.InitService()
	if err != nil {
		return
	}
	client.LaunchTunnels()
	return
}
