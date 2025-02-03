package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	f, _ := os.Create("test")

	for i := 0; i < 10; i++ {
		f.WriteString(strconv.Itoa(i))
	}

	f.Seek(0, 0)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
}
