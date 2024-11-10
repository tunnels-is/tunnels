package main

import "fmt"

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

func main() {
	m := [2]byte{1, 1}
	n := [2]byte{1, 2}
	if m == n {
		fmt.Println("YES!")
	} else {
		fmt.Println("NO")
	}
}
