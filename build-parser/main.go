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

		fmt.Println(path)
		fb, err := os.ReadFile(path)
		fbs := bytes.Split(fb, []byte{10})
		for i := range fbs {
			if path == "../setcap/setcap.go" {
				continue
			}
			if bytes.Contains(fbs[i], filter) {
				if path == "../client/logging.go" && i == 144 {
					if bytes.Contains(fbs[i], []byte("fmt.Println(line)")) {
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
