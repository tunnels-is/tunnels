package service

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/client"
)

func Start(minimal bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, string(debug.Stack()))
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error
	if minimal {
		err = client.InitMinimalService()
	} else {
		err = client.InitService()
	}
	if err != nil {
		time.Sleep(5 * time.Second)
		panic(err)
	}
	if minimal {
		client.LaunchMinimalTunnels()
	} else {
		client.LaunchTunnels()
	}
}

func StartWithExternalMonitor(ctx context.Context, minimal bool, id int, monitor chan int) {
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
	var err error
	if minimal {
		err = client.InitMinimalService()
	} else {
		err = client.InitService()
	}
	if err != nil {
		fmt.Println("Error initializing tunnels service:", err)
		return
	}
	if minimal {
		client.LaunchMinimalTunnels()
	} else {
		client.LaunchTunnels()
	}
}
