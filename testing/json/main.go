package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

var x sync.Map

func main() {
	x.Store("mewo", "meow")
	x.Store("mewo2", "meow")
	x.Store("mewo1", "meow")
	x.Store("mewo3", "meow")

	xx, err := json.Marshal(&x)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(xx))
}
