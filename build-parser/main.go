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
	checkDir("../setcap")
	if fmtCount > 0 {
		panic("YOU HAVE DEBUG PRINTS IN THE BUILD")
	}
}

func checkDir(dir string) {
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if path == "../setcap/setcap.go" {
			return nil
		}

		fmt.Println(path)
		fb, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fbs := bytes.Split(fb, []byte{10})
		for i := range fbs {
			if bytes.Contains(fbs[i], filter) {

				if path == "../client/logging.go" && bytes.Contains(fbs[i], []byte("fmt.Println(line)")) {
					continue
				}
				if path == "../server/main.go" && bytes.Contains(fbs[i], []byte("fmt.Println(version.Version)")) {
					continue
				}
				fmt.Println(i, string(fbs[i]))
				fmtCount++
			}
		}
		return nil
	})
}
