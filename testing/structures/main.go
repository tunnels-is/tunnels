package main

import (
	"fmt"
	"net"
	"runtime"
	"time"
)

type xc struct {
	IP string
}

var x [][][][]*xc

// func addMapping(o1, o2, o3, o4 int, client *xc) {
// 	if len(x[o1]) == 0 {
// 		x[o1] = make([][][]*xc, 256)
// 	}
// 	if len(x[o1][o2]) == 0 {
// 		x[o1][o2] = make([][]*xc, 256)
// 	}
// 	if len(x[o1][o2][o3]) == 0 {
// 		x[o1][o2][o3] = make([]*xc, 256)
// 	}
// }

func alloc() {
	x = make([][][][]*xc, 255)
	fmt.Println(x[100])
}

var testMap = make(map[[4]byte]struct{})

func main() {
	ip, VPLNetwork, err := net.ParseCIDR("10.0.0.0/16")
	if err != nil {
		panic(err)
	}

	ip = ip.Mask(VPLNetwork.Mask)

	for VPLNetwork.Contains(ip) {
		testMap[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = struct{}{}
		inc(ip)
	}

	for {
		time.Sleep(1 * time.Second)
		PrintMemUsage()
		fmt.Println(len(testMap))

	}
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Current = %v MiB", m.Alloc/1024/1024)
	fmt.Printf("\tTotal(over time) = %v MiB", m.TotalAlloc/1024/1024)
	fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
	fmt.Printf("\tGC count = %v\n", m.NumGC)
}
