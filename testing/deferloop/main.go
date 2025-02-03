package main

import "fmt"

func main() {
	for i := 0; i < 10; i++ {
		defer func(ii int) {
			fmt.Println(ii)
		}(i)
		fmt.Println(i)
	}
}
