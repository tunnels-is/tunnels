package service

import (
	"runtime"
	"runtime/debug"

	"github.com/tunnels-is/tunnels/core"
)

func Start() {
	defer func() {
		if r := recover(); r != nil {
			core.ERROR(r, string(debug.Stack()))
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	err := core.InitService()
	if err != nil {
		panic(err)
	}
	core.LaunchEverything()
}

func StartMinimal() {
	defer func() {
		if r := recover(); r != nil {
			core.ERROR(r, string(debug.Stack()))
		}
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())
	err := core.InitMinimal()
	if err != nil {
		panic(err)
	}
	core.LaunchMinimal()
}
