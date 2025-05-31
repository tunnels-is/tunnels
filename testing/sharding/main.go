package main

import (
	"fmt"
	"hash/crc32"

	"github.com/google/uuid"
)

var numShards = 5

func main() {

	m := make(map[string]int)
	num := 1000000
	for range num {
		uid := uuid.NewString()
		m[uid] = getShardIndex(uid)

	}

	cm := make(map[int]int)

	for _, v := range m {
		cm[v]++
	}

	for i, v := range cm {
		fmt.Println(i, v)
	}

}

// getShardIndex determines the target shard index (0 to numShards-1) for a given key.
func getShardIndex(key string) int {
	checksum := crc32.ChecksumIEEE([]byte(key))
	return int(checksum % uint32(numShards))
}
