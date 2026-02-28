// wg-db-setup patches the WireGuard fields on a server record in the BoltDB.
// Usage: wg-db-setup -db /opt/tunnels/tunnels.db -ip 74.63.223.157 \
//                    -privkey <b64> -port 442 -base http://127.0.0.1:8181
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	gobolt "go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/curve25519"
)

// Minimal server shape — matches types.Server JSON tags exactly.
type serverRecord struct {
	ID              primitive.ObjectID   `json:"_id"`
	Tag             string               `json:"Tag"`
	Country         string               `json:"Country"`
	IP              string               `json:"IP"`
	Port            string               `json:"Port"`
	Groups          []primitive.ObjectID `json:"Groups"`
	WireGuardPort   string               `json:"WireGuardPort,omitempty"`
	WireGuardPubKey string               `json:"WireGuardPubKey,omitempty"`
	WGBaseURL       string               `json:"WGBaseURL,omitempty"`
}

func main() {
	dbPath  := flag.String("db",      "/opt/tunnels/tunnels.db",  "path to BoltDB file")
	targetIP := flag.String("ip",     "",                          "server IP to update")
	privKeyB64 := flag.String("privkey", "",                       "wg-server base64 private key (to derive pubkey)")
	port    := flag.String("port",    "",                          "WireGuard port (e.g. 442)")
	baseURL := flag.String("base",    "http://127.0.0.1:8181",    "wg-server management base URL")
	listOnly := flag.Bool("list",     false,                       "list server records and exit")
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
				fmt.Printf("id=%-26s  ip=%-18s  port=%-6s  wgkey=%s  wgbase=%s\n",
					string(k), s.IP, s.Port, s.WireGuardPubKey, s.WGBaseURL)
				return nil
			})
		})
		return
	}

	if *targetIP == "" || *privKeyB64 == "" || *port == "" {
		log.Fatal("flags -ip, -privkey, and -port are required (or use -list)")
	}

	// Derive public key from private key.
	privBytes, err := base64.StdEncoding.DecodeString(*privKeyB64)
	if err != nil || len(privBytes) != 32 {
		log.Fatalf("invalid privkey: %v", err)
	}
	pubBytes, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		log.Fatalf("derive pubkey: %v", err)
	}
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubBytes)
	fmt.Printf("derived pubkey: %s\n", pubKeyB64)

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
			s.WireGuardPubKey = pubKeyB64
			s.WireGuardPort   = *port
			s.WGBaseURL       = *baseURL
			data, err := json.Marshal(s)
			if err != nil {
				return err
			}
			if err := b.Put(k, data); err != nil {
				return err
			}
			fmt.Printf("updated server: id=%s ip=%s wgport=%s wgbase=%s\n",
				string(k), s.IP, s.WireGuardPort, s.WGBaseURL)
			return nil
		})
	})
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	fmt.Println("done")
}
