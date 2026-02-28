//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	gobolt "go.etcd.io/bbolt"
)

func main() {
	dbPath := flag.String("db", "/opt/tunnels/tunnels.db", "path to BoltDB file")
	bucket := flag.String("bucket", "servers", "bucket to dump")
	flag.Parse()

	db, err := gobolt.Open(*dbPath, 0o600, &gobolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_ = db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		if b == nil {
			fmt.Printf("bucket %q not found\n", *bucket)
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var m map[string]any
			if err := json.Unmarshal(v, &m); err != nil {
				fmt.Printf("key=%s  parse_err=%v\n", k, err)
				return nil
			}
			out, _ := json.MarshalIndent(m, "", "  ")
			fmt.Printf("=== key=%s ===\n%s\n\n", k, out)
			return nil
		})
	})
}
