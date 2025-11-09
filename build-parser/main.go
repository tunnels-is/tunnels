package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	filter   = []byte("fmt.Println")
	fmtCount = 0
)

func main() {
	checkDir("../server")
	checkDir("../client")
	checkDir("../certs")
	checkDir("../iptables")
	checkDir("../setcap")
	if fmtCount > 0 {
		panic("YOU HAVE DEBUG PRINTS IN THE BUILD")
	}
}

func checkDir(dir string) {
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".crt") {
			return nil
		}
		if strings.HasSuffix(path, ".dll") {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.HasSuffix(path, "server") {
			return nil
		}
		if path == "../setcap/setcap.go" {
			return nil
		}

		fmt.Println(path)
		fb, err := os.ReadFile(path)
		fbs := bytes.Split(fb, []byte{10})
		for i := range fbs {
			if bytes.Contains(fbs[i], filter) {
				if path == "../client/logging.go" && i == 157 {
					if bytes.Contains(fbs[i], []byte("fmt.Println(line)")) {
						continue
					}
				}
				if path == "../client/update.go" && i == 35 {
					if bytes.Contains(fbs[i], []byte("fmt.Println(s...)")) {
						continue
					}
				}
				if path == "../server/main.go" && i == 142 {
					if bytes.Contains(fbs[i], []byte("fmt.Println(version.Version)")) {
						continue
					}
				}
				fmt.Println(i, string(fbs[i]))
				fmtCount++
			}
		}
		return nil
	})
}
