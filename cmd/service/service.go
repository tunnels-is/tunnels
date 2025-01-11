package service

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/core"
)

func Start() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, string(debug.Stack()))
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	err := core.InitService()
	if err != nil {
		time.Sleep(5 * time.Second)
		panic(err)
	}
	core.LaunchEverything()
}

func StartWithExternalMonitor(ctx context.Context, id int, monitor chan int) {
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
	err := core.InitService()
	if err != nil {
		panic(err)
	}
	core.LaunchEverything()
}
