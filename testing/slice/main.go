package main

import (
	"fmt"
	"slices"
	"strconv"
	"time"
)

type target struct {
	Name string
}

var targets []*target

func main() {
	for i := 0; i < 5000; i++ {
		targets = append(targets, &target{Name: strconv.Itoa(i)})
	}
	go remove()

	for _, v := range targets {
		if v == nil {
			continue
		}
		time.Sleep(2 * time.Millisecond)
		fmt.Println(v.Name)
	}
}

func remove() {
	for {
		time.Sleep(1 * time.Millisecond)
		targets = slices.Delete(targets, 0, 1)
	}
}
