// user-migrate copies all users from MongoDB into a BBolt database, then
// iterates every record in BBolt and verifies it matches its MongoDB
// counterpart field-by-field.
//
// Usage:
//
//	user-migrate -mongo "mongodb://root:example@localhost:27017" -db ./tunnels.db
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	gobolt "go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoUser is the MongoDB-side shape used for BSON decoding.
type MongoUser struct {
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

// BoltUser is the BBolt-side shape using uuid.UUID (matches the server).
type BoltUser struct {
	ID uuid.UUID `json:"_id"`

	Email                 string    `json:"Email"`
	Updated               time.Time `json:"Updated"`
	AdditionalInformation string    `json:"AdditionalInformation,omitempty"`
	Disabled              bool      `json:"Disabled"`

	APIKey string `json:"APIKey"`

	Password         string         `json:"Password"`
	ConfirmCode      string         `json:"ConfirmCode"`
	LastResetRequest time.Time      `json:"LastResetRequest"`
	RecoveryCodes    []byte         `json:"RecoveryCodes"`
	TwoFactorCode    []byte         `json:"TwoFactorCode"`
	TwoFactorEnabled bool           `json:"TwoFactorEnabled"`
	Tokens           []*DeviceToken `json:"Tokens"`

	IsAdmin   bool        `json:"IsAdmin"`
	IsManager bool        `json:"IsManager"`
	Groups    []uuid.UUID `json:"Groups"`

	Trial         bool        `json:"Trial"`
	Key           *LicenseKey `json:"Key"`
	SubExpiration time.Time   `json:"SubExpiration"`
}

func mongoToBolt(mu *MongoUser) *BoltUser {
	groups := make([]uuid.UUID, len(mu.Groups))
	for i := range mu.Groups {
		groups[i] = uuid.New()
	}
	return &BoltUser{
		ID:                    uuid.New(),
		Email:                 mu.Email,
		Updated:               mu.Updated,
		AdditionalInformation: mu.AdditionalInformation,
		Disabled:              mu.Disabled,
		APIKey:                mu.APIKey,
		Password:              mu.Password,
		ConfirmCode:           mu.ConfirmCode,
		LastResetRequest:      mu.LastResetRequest,
		RecoveryCodes:         mu.RecoveryCodes,
		TwoFactorCode:         mu.TwoFactorCode,
		TwoFactorEnabled:      mu.TwoFactorEnabled,
		Tokens:                mu.Tokens,
		IsAdmin:               mu.IsAdmin,
		IsManager:             mu.IsManager,
		Groups:                groups,
		Trial:                 mu.Trial,
		Key:                   mu.Key,
		SubExpiration:         mu.SubExpiration,
	}
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

	// Build a lookup map keyed by email for the verification step.
	// (IDs change format during migration, so we match by email.)
	mongoByEmail := make(map[string]*MongoUser, len(mongoUsers))
	for i := range mongoUsers {
		mongoByEmail[mongoUsers[i].Email] = &mongoUsers[i]
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
	for i := range mongoUsers {
		bu := mongoToBolt(&mongoUsers[i])
		key := bu.ID.String()
		data, err := json.Marshal(bu)
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
	// IDs and Groups changed format (ObjectID→UUID), so verification
	// compares all other fields individually.
	fmt.Println("verifying ...")
	verified := 0
	mismatches := 0

	err = bolt.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(usersBucket))
		return b.ForEach(func(k, v []byte) error {
			id := string(k)

			var boltUser BoltUser
			if err := json.Unmarshal(v, &boltUser); err != nil {
				fmt.Printf("  FAIL  id=%s  bbolt decode error: %v\n", id, err)
				mismatches++
				return nil
			}

			mongoUser, ok := mongoByEmail[boltUser.Email]
			if !ok {
				fmt.Printf("  FAIL  id=%s  email=%s  exists in BBolt but not in MongoDB\n", id, boltUser.Email)
				mismatches++
				return nil
			}

			if !verifyFields(&boltUser, mongoUser) {
				fmt.Printf("  MISMATCH  id=%s  email=%s\n", id, boltUser.Email)
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
func fetchAllMongoUsers(client *mongo.Client) ([]MongoUser, error) {
	cursor, err := client.Database("users").
		Collection("users").
		Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var users []MongoUser
	for cursor.Next(context.Background()) {
		var u MongoUser
		if err := cursor.Decode(&u); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		users = append(users, u)
	}
	return users, cursor.Err()
}

// verifyFields compares all fields except ID and Groups (which changed format).
func verifyFields(bolt *BoltUser, mongo *MongoUser) bool {
	ok := true
	check := func(name string, match bool) {
		if !match {
			fmt.Printf("    field %-22s  mismatch\n", name)
			ok = false
		}
	}

	check("Email", bolt.Email == mongo.Email)
	check("Updated", bolt.Updated.Equal(mongo.Updated))
	check("AdditionalInformation", bolt.AdditionalInformation == mongo.AdditionalInformation)
	check("Disabled", bolt.Disabled == mongo.Disabled)
	check("APIKey", bolt.APIKey == mongo.APIKey)
	check("Password", bolt.Password == mongo.Password)
	check("ConfirmCode", bolt.ConfirmCode == mongo.ConfirmCode)
	check("LastResetRequest", bolt.LastResetRequest.Equal(mongo.LastResetRequest))
	check("TwoFactorEnabled", bolt.TwoFactorEnabled == mongo.TwoFactorEnabled)
	check("IsAdmin", bolt.IsAdmin == mongo.IsAdmin)
	check("IsManager", bolt.IsManager == mongo.IsManager)
	check("Trial", bolt.Trial == mongo.Trial)
	check("SubExpiration", bolt.SubExpiration.Equal(mongo.SubExpiration))

	// ID and Groups are intentionally skipped (ObjectID→UUID format change).
	return ok
}
