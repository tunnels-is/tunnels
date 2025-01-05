package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	x := make(map[[4]byte]bool)
	xl := sync.Mutex{}
	x2 := make([][4]byte, 0)

	count := 100
	for i := 0; i < count; i++ {
		x[[4]byte{1, 1, 1, byte(i)}] = true
	}
	for i := 0; i < count; i++ {
		x2 = append(x2, [4]byte{1, 1, 1, byte(i)})
	}

	s := time.Now()
	xl.Lock()
	_, ok := x[[4]byte{1, 1, 1, 1}]
	if ok {
		ok = false
	}
	xl.Unlock()
	end := time.Since(s).Nanoseconds()
	fmt.Println("Map", end)

	s = time.Now()
	ok = false
	for _, v := range x2 {
		if v == [4]byte{1, 1, 1, 1} {
			ok = true
		}
	}
	end = time.Since(s).Nanoseconds()
	fmt.Println("Slice", end)
}
