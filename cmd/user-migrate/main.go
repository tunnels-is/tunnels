// user-migrate copies all users from MongoDB into a BBolt database, then
// iterates every record in BBolt and verifies it matches its MongoDB
// counterpart field-by-field.
//
// Usage:
//
//	user-migrate -mongo "mongodb://root:example@localhost:27017" -db ./tunnels.db
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	gobolt "go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User mirrors the server-side User struct (server/types.go).
// JSON tags must match exactly because BBolt stores records as JSON.
type User struct {
	ID primitive.ObjectID `json:"_id" bson:"_id"`

	Email                 string    `json:"Email" bson:"Email"`
	Updated               time.Time `json:"Updated" bson:"Updated"`
	AdditionalInformation string    `json:"AdditionalInformation,omitempty" bson:"AdditionalInformation"`
	Disabled              bool      `json:"Disabled" bson:"Disabled"`

	APIKey string `json:"APIKey" bson:"APIKey"`

	Password         string         `json:"Password" bson:"Password"`
	ConfirmCode      string         `json:"ConfirmCode" bson:"ConfirmCode"`
	LastResetRequest time.Time      `json:"LastResetRequest" bson:"LastResetRequest"`
	RecoveryCodes    []byte         `json:"RecoveryCodes" bson:"RecoveryCodes"`
	TwoFactorCode    []byte         `json:"TwoFactorCode" bson:"TwoFactorCode"`
	TwoFactorEnabled bool           `json:"TwoFactorEnabled" bson:"TwoFactorEnabled"`
	Tokens           []*DeviceToken `json:"Tokens" bson:"Tokens"`

	IsAdmin   bool                 `json:"IsAdmin" bson:"IsAdmin"`
	IsManager bool                 `json:"IsManager" bson:"IsManager"`
	Groups    []primitive.ObjectID `json:"Groups" bson:"Groups"`

	Trial         bool        `json:"Trial" bson:"Trial"`
	Key           *LicenseKey `json:"Key" bson:"Key"`
	SubExpiration time.Time   `json:"SubExpiration" bson:"SubExpiration"`
}

type DeviceToken struct {
	DT      string    `json:"DT" bson:"DT"`
	N       string    `json:"N" bson:"N"`
	Created time.Time `json:"C" bson:"C"`
}

type LicenseKey struct {
	Created time.Time `json:"Created" bson:"Created"`
	Months  int       `json:"Months" bson:"Months"`
	Key     string    `json:"Key" bson:"Key"`
}

const usersBucket = "users"

func main() {
	mongoURI := flag.String("mongo", "mongodb://root:example@localhost:27017", "MongoDB connection URI")
	dbPath := flag.String("db", "./tunnels.db", "path to BBolt database file")
	flag.Parse()

	// ── 0. Connect to MongoDB ───────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mOpts := options.Client().ApplyURI(*mongoURI).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, mOpts)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer client.Disconnect(context.Background())

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}
	fmt.Println("connected to MongoDB")

	// ── 1. Read all users from MongoDB ──────────────────────────────
	mongoUsers, err := fetchAllMongoUsers(client)
	if err != nil {
		log.Fatalf("fetch mongo users: %v", err)
	}
	fmt.Printf("found %d user(s) in MongoDB\n", len(mongoUsers))
	if len(mongoUsers) == 0 {
		fmt.Println("nothing to migrate")
		return
	}

	// Build a lookup map keyed by hex ID for the verification step.
	mongoByID := make(map[string]*User, len(mongoUsers))
	for i := range mongoUsers {
		mongoByID[mongoUsers[i].ID.Hex()] = &mongoUsers[i]
	}

	// ── 2. Open BBolt and write all users ───────────────────────────
	bolt, err := gobolt.Open(*dbPath, 0o600, &gobolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		log.Fatalf("bbolt open: %v", err)
	}
	defer bolt.Close()

	if err := bolt.Update(func(tx *gobolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(usersBucket))
		return err
	}); err != nil {
		log.Fatalf("create bucket: %v", err)
	}

	written := 0
	for _, u := range mongoUsers {
		key := u.ID.Hex()
		data, err := json.Marshal(u)
		if err != nil {
			log.Fatalf("marshal %s: %v", key, err)
		}
		if err := bolt.Update(func(tx *gobolt.Tx) error {
			return tx.Bucket([]byte(usersBucket)).Put([]byte(key), data)
		}); err != nil {
			log.Fatalf("bbolt put %s: %v", key, err)
		}
		written++
	}
	fmt.Printf("wrote %d user(s) to BBolt\n", written)

	// ── 3. Iterate BBolt and verify against MongoDB ─────────────────
	fmt.Println("verifying ...")
	verified := 0
	mismatches := 0

	err = bolt.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(usersBucket))
		return b.ForEach(func(k, v []byte) error {
			id := string(k)

			// Decode the BBolt record.
			var boltUser User
			if err := json.Unmarshal(v, &boltUser); err != nil {
				fmt.Printf("  FAIL  id=%s  bbolt decode error: %v\n", id, err)
				mismatches++
				return nil
			}

			// Find the corresponding MongoDB user.
			mongoUser, ok := mongoByID[id]
			if !ok {
				fmt.Printf("  FAIL  id=%s  exists in BBolt but not in MongoDB\n", id)
				mismatches++
				return nil
			}

			// Re-marshal both through JSON so the comparison is
			// format-identical (same serialisation path BBolt uses).
			boltJSON, err := json.Marshal(boltUser)
			if err != nil {
				fmt.Printf("  FAIL  id=%s  re-marshal bbolt: %v\n", id, err)
				mismatches++
				return nil
			}
			mongoJSON, err := json.Marshal(mongoUser)
			if err != nil {
				fmt.Printf("  FAIL  id=%s  marshal mongo: %v\n", id, err)
				mismatches++
				return nil
			}

			if !bytes.Equal(boltJSON, mongoJSON) {
				fmt.Printf("  MISMATCH  id=%s  email=%s\n", id, boltUser.Email)
				diffFields(&boltUser, mongoUser)
				mismatches++
				return nil
			}

			verified++
			return nil
		})
	})
	if err != nil {
		log.Fatalf("bbolt verify scan: %v", err)
	}

	fmt.Printf("field verification: %d ok, %d mismatches\n", verified, mismatches)
	if mismatches > 0 {
		log.Fatal("MIGRATION FAILED: mismatches detected")
	}

	// ── 4. Count verification ───────────────────────────────────────
	boltCount := 0
	_ = bolt.View(func(tx *gobolt.Tx) error {
		boltCount = tx.Bucket([]byte(usersBucket)).Stats().KeyN
		return nil
	})
	mongoCount := len(mongoUsers)

	fmt.Printf("count verification: mongo=%d bbolt=%d\n", mongoCount, boltCount)
	if boltCount != mongoCount {
		log.Fatalf("MIGRATION FAILED: count mismatch (mongo=%d, bbolt=%d)", mongoCount, boltCount)
	}

	fmt.Println("all users migrated and verified successfully")
}

// fetchAllMongoUsers returns every document in the users.users collection.
func fetchAllMongoUsers(client *mongo.Client) ([]User, error) {
	cursor, err := client.Database("users").
		Collection("users").
		Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var users []User
	for cursor.Next(context.Background()) {
		var u User
		if err := cursor.Decode(&u); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		users = append(users, u)
	}
	return users, cursor.Err()
}

// diffFields prints per-field differences between two User values.
func diffFields(bolt, mongo *User) {
	check := func(name string, b, m any) {
		bj, _ := json.Marshal(b)
		mj, _ := json.Marshal(m)
		if !bytes.Equal(bj, mj) {
			fmt.Printf("    field %-22s  bbolt=%s  mongo=%s\n", name, bj, mj)
		}
	}

	check("ID", bolt.ID, mongo.ID)
	check("Email", bolt.Email, mongo.Email)
	check("Updated", bolt.Updated, mongo.Updated)
	check("AdditionalInformation", bolt.AdditionalInformation, mongo.AdditionalInformation)
	check("Disabled", bolt.Disabled, mongo.Disabled)
	check("APIKey", bolt.APIKey, mongo.APIKey)
	check("Password", bolt.Password, mongo.Password)
	check("ConfirmCode", bolt.ConfirmCode, mongo.ConfirmCode)
	check("LastResetRequest", bolt.LastResetRequest, mongo.LastResetRequest)
	check("RecoveryCodes", bolt.RecoveryCodes, mongo.RecoveryCodes)
	check("TwoFactorCode", bolt.TwoFactorCode, mongo.TwoFactorCode)
	check("TwoFactorEnabled", bolt.TwoFactorEnabled, mongo.TwoFactorEnabled)
	check("Tokens", bolt.Tokens, mongo.Tokens)
	check("IsAdmin", bolt.IsAdmin, mongo.IsAdmin)
	check("IsManager", bolt.IsManager, mongo.IsManager)
	check("Groups", bolt.Groups, mongo.Groups)
	check("Trial", bolt.Trial, mongo.Trial)
	check("Key", bolt.Key, mongo.Key)
	check("SubExpiration", bolt.SubExpiration, mongo.SubExpiration)
}
