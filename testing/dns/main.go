package main

import (
	"fmt"

	"github.com/tunnels-is/tunnels/certs"
)

func main() {
	fmt.Println(certs.ResolveMetaTXT("cert-test.tunnels.is"))
}
