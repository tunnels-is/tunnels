package service

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/core"
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
		err = core.InitMinimalService()
	} else {
		err = core.InitService()
	}
	if err != nil {
		time.Sleep(5 * time.Second)
		panic(err)
	}
	if minimal {
		core.LaunchMinimalTunnels()
	} else {
		core.LaunchTunnels()
	}
}

func StartWithExternalMonitor(ctx context.Context, minimal bool, id int, monitor chan int) {
	defer func() {
		if r := recover(); r != nil {
			core.ERROR(r, string(debug.Stack()))
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
		err = core.InitMinimalService()
	} else {
		err = core.InitService()
	}
	if err != nil {
		fmt.Println("Error initializing tunnels service:", err)
		return
	}
	if minimal {
		core.LaunchMinimalTunnels()
	} else {
		core.LaunchTunnels()
	}
}
