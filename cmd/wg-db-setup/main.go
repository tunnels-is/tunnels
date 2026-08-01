package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
	gobolt "go.etcd.io/bbolt"
)

type serverRecord struct {
	ID              uuid.UUID   `json:"_id"`
	Tag             string      `json:"Tag"`
	Country         string      `json:"Country"`
	IP              string      `json:"IP"`
	Port            string      `json:"Port"`
	Groups          []uuid.UUID `json:"Groups"`
	WireGuardPort   int         `json:"WireGuardPort,omitempty"`
	WireGuardPubKey string      `json:"WireGuardPubKey,omitempty"`
	WGBaseURL       string      `json:"WGBaseURL,omitempty"`
}

func main() {
	dbPath := flag.String("db", "/opt/tunnels/tunnels.db", "path to BoltDB file")
	targetIP := flag.String("ip", "", "server IP to update")
	pubKeyB64 := flag.String("pubkey", "", "wg-server base64 public key")
	port := flag.String("port", "", "WireGuard port (e.g. 442)")
	baseURL := flag.String("base", "http://127.0.0.1:8181", "wg-server management base URL")
	listOnly := flag.Bool("list", false, "list server records and exit")
	flag.Parse()

	db, err := gobolt.Open(*dbPath, 0o600, &gobolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if *listOnly {
		_ = db.View(func(tx *gobolt.Tx) error {
			b := tx.Bucket([]byte("servers"))
			if b == nil {
				fmt.Println("no servers bucket")
				return nil
			}
			return b.ForEach(func(k, v []byte) error {
				var s serverRecord
				if err := json.Unmarshal(v, &s); err != nil {
					fmt.Printf("key=%s  parse_err=%v\n", k, err)
					return nil
				}
				fmt.Printf("id=%-26s  ip=%-18s  port=%-6s  wgport=%-6d  wgkey=%s  wgbase=%s\n",
					string(k), s.IP, s.Port, s.WireGuardPort, s.WireGuardPubKey, s.WGBaseURL)
				return nil
			})
		})
		return
	}

	if *targetIP == "" || *pubKeyB64 == "" || *port == "" {
		log.Fatal("flags -ip, -pubkey, and -port are required (or use -list)")
	}

	err = db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte("servers"))
		if b == nil {
			return fmt.Errorf("no servers bucket")
		}
		return b.ForEach(func(k, v []byte) error {
			var s serverRecord
			if err := json.Unmarshal(v, &s); err != nil {
				return nil
			}
			if s.IP != *targetIP {
				return nil
			}
			s.WireGuardPubKey = *pubKeyB64
			portN, perr := strconv.Atoi(*port)
			if perr != nil {
				return fmt.Errorf("invalid -port %q: %w", *port, perr)
			}
			s.WireGuardPort = portN
			s.WGBaseURL = *baseURL
			data, err := json.Marshal(s)
			if err != nil {
				return err
			}
			if err := b.Put(k, data); err != nil {
				return err
			}
			fmt.Printf("updated server: id=%s ip=%s wgport=%d wgbase=%s\n",
				string(k), s.IP, s.WireGuardPort, s.WGBaseURL)
			return nil
		})
	})
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	fmt.Println("done")
}
