package service

import (
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
