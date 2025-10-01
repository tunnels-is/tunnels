package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tunnels-is/tunnels/argon"
)

func main() {
	preHash := ""
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	wdl := strings.Split(wd, string(os.PathSeparator))
	preHash += wdl[len(wdl)-1]
	// fmt.Println(preHash)

	ex, err := os.Executable()
	edl := strings.Split(ex, string(os.PathSeparator))
	preHash += edl[len(edl)-1]
	// fmt.Println(preHash)
	preHash += "v0.0.1"
	// fmt.Println(preHash)

	a := &argon.Argon{
		Memory:      20 * 1024, // 20 MiB
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
	hx, err := a.Hash(preHash)
	fmt.Println("HX:", hx)
	key, err := a.Key(preHash, true)
	fmt.Println("KEY:", string(key))
	key2, err := a.Key(preHash, false)
	fmt.Println("KEY:", string(key2))
	ok, err := a.Compare(preHash, hx)
	fmt.Println("COMP:", ok, err)
	ok, err = a.Compare(preHash+" ", hx)
	fmt.Println("COMP:", ok, err)
}
