package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed admin_dist
var adminDist embed.FS

func adminUIHandler() http.Handler {
	fsys, err := fs.Sub(adminDist, "admin_dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsys))
}
