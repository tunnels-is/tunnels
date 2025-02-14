package main

import (
	"fmt"
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

//		10.0.255.255

func alloc() {
	x = make([][][][]*xc, 256)
	x[10] = make([][][]*xc, 11)
	x[10][0] = make([][]*xc, 256)

	for ii := 0; ii < 256; ii++ {
		x[10][0][ii] = make([]*xc, 256)

		for iii := 0; iii < 256; iii++ {
			x[10][0][ii][iii] = &xc{IP: fmt.Sprintf("%d.%d.%d.%d", 10, 0, ii, iii)}
		}
	}
}

var testMap = make(map[[4]byte]struct{})

func main() {
	alloc()

	count := 1
	for {
		time.Sleep(1 * time.Second)
		if count > 200 {
			count = 1
		}
		count++

		fmt.Println(x[10][0][0][1])
		fmt.Println(x[10][0][255][255])
		PrintMemUsage()
		fmt.Println(len(testMap))

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
