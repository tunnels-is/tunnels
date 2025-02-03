package main

import (
	"crypto/md5"
	"fmt"
	"sync"
)

var (
	seenLogs = make(map[[md5.Size]byte]bool)
	logMutex = &sync.Mutex{}
)

func logUnique(message string) {
	// Compute the MD5 hash of the log message
	hash := md5.Sum([]byte(message))

	// Check if we've seen this hash before
	logMutex.Lock()
	_, exists := seenLogs[hash]
	if !exists {
		seenLogs[hash] = true
		fmt.Println(message) // Log the message if it's unique
	}
	logMutex.Unlock()
}

func main() {
	logUnique("This is a unique log message")
	logUnique("This is another unique log message")
	logUnique("This is a unique log message") // This won't be printed again
}
