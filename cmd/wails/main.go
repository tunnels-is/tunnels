package main

import (
	"github.com/wailsapp/wails/v3"
	"github.com/tunnels-is/tunnels/core"
)

func main() {
	app := wails.CreateApp(&wails.AppConfig{
		Title:            "Tunnels",
		Width:            1024,
		Height:           768,
		Frameless:        true,
		DisableResize:    true,
		FullscreenButton: false,
	})
	app.Bind(core.InitService)
	app.Run()
}