package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/tunnels-is/tunnels/coretwo/internal/platform"
)

func main() {
	fmt.Printf("Testing platform package on %s\n", runtime.GOOS)

	ctx := context.Background()
	err := platform.Initialize(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Platform initialization failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Platform initialization successful")

	err = platform.CheckAdmin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Admin check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Admin check successful")

	err = platform.InitializeNetwork(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network initialization failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Network initialization successful")
}
