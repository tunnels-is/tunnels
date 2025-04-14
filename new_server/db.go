package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dgraph-io/badger/v4"
)

var stores = make([]*badger.DB, numShards)
var indexes = make([]*badger.DB, numShards)

// var groupDB *badger.DB
// var serverDB *badger.DB
// var tokenDB *badger.DB
// var indexDB *badger.DB // For username -> user UUID mapping

const (
	userPrefix   = "u:"
	groupPrefix  = "g:"
	serverPrefix = "s:"
	tokenPrefix  = "t:"
	indexPrefix  = "idx_usrname:"
)

func initDBs(logger *slog.Logger) error {
	var err error

	openOpts := badger.DefaultOptions("").WithLogger(nil) // Badger logs are verbose, manage logging externally

	for i := range numShards {
		stores[i], err = badger.Open(openOpts.WithDir("shard" + strconv.Itoa(i) + ".db").WithValueDir("shard" + strconv.Itoa(i) + ".db"))
		if err != nil {
			logger.Error("Failed to open user DB", slog.Any("error", err))
			return fmt.Errorf("failed to open user DB: %w", err)
		}
	}

	for i := range numShards {
		indexes[i], err = badger.Open(openOpts.WithDir("index_shard" + strconv.Itoa(i) + ".db").WithValueDir("index_shard" + strconv.Itoa(i) + ".db"))
		if err != nil {
			logger.Error("Failed to open user DB", slog.Any("error", err))
			return fmt.Errorf("failed to open user DB: %w", err)
		}
	}

	logger.Info("Databases initialized successfully")
	return nil
}

func closeDBs(logger *slog.Logger) {
	for _, db := range stores {
		if db != nil {
			if err := db.Close(); err != nil {
				logger.Error(fmt.Sprintf("Error closing DB"), slog.Any("error", err))
			}
		}
	}

	for _, db := range indexes {
		if db != nil {
			if err := db.Close(); err != nil {
				logger.Error(fmt.Sprintf("Error closing DB"), slog.Any("error", err))
			}
		}
	}
	logger.Info("Databases closed")
}

// --- Generic Helpers ---

func createItem(db *badger.DB, key []byte, value any) error {
	valBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for key %s: %w", string(key), err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		// Optionally check if key already exists if true 'create' is needed
		// _, err := txn.Get(key)
		// if err == nil {
		// 	return errors.New("item already exists") // Or badger.ErrKeyExists?
		// }
		// if err != badger.ErrKeyNotFound {
		//  return err // Handle other potential errors
		// }
		return txn.Set(key, valBytes)
	})
	return err
}

func getItem(db *badger.DB, key []byte, target any) error {
	var valCopy []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err // Handles ErrKeyNotFound implicitly
		}

		valCopy, err = item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to copy value for key %s: %w", string(key), err)
		}
		return nil
	})

	if err != nil {
		return err // e.g., badger.ErrKeyNotFound
	}

	if err := json.Unmarshal(valCopy, target); err != nil {
		return fmt.Errorf("failed to unmarshal value for key %s: %w", string(key), err)
	}
	return nil
}

func updateItem(db *badger.DB, key []byte, value any) error {
	valBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for update key %s: %w", string(key), err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		// Check if key exists to ensure it's an update
		_, err := txn.Get(key)
		if err != nil {
			return err // Return ErrKeyNotFound if it doesn't exist
		}
		return txn.Set(key, valBytes)
	})
	return err
}

func deleteItem(db *badger.DB, key []byte) error {
	err := db.Update(func(txn *badger.Txn) error {
		// Optionally check existence first
		// _, err := txn.Get(key)
		// if err != nil {
		// 	 return err // Propagate ErrKeyNotFound or other errors
		// }
		return txn.Delete(key)
	})
	// Badger's Delete doesn't error if the key doesn't exist, adjust if strict check needed
	return err
}

// List items might be better implemented specifically per type
// due to unmarshaling requirements. Returning [][]byte avoids generics
// but pushes unmarshaling to the caller.

func listItemsRaw(db *badger.DB, prefix []byte) ([][]byte, error) {
	var items [][]byte
	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				// Decide whether to skip or fail all
				return fmt.Errorf("failed to copy value during list for prefix %s: %w", string(prefix), err)
			}
			items = append(items, valCopy)
		}
		return nil
	})
	return items, err
}

// --- User Specific ---

func saveUser(user *User) error {
	key := userPrefix + user.UUID
	dbID := getShardIndex(key)
	return createItem(stores[dbID], []byte(key), user)
}

func getUser(uuid string) (*User, error) {
	var user User
	key := userPrefix + uuid
	dbID := getShardIndex(key)
	err := getItem(stores[dbID], []byte(key), &user)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("user %s not found: %w", uuid, errNotFound)
		}
		return nil, err
	}
	return &user, nil
}

func getUserByGoogleID(googleID string) (*User, error) {
	var foundUser *User
	key := userPrefix + googleID
	dbID := getShardIndex(key)
	err := stores[dbID].View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(userPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(userPrefix)); it.ValidForPrefix([]byte(userPrefix)); it.Next() {
			item := it.Item()
			var user User
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &user)
			})
			if err != nil {
				// Log this error but continue scanning
				logger.Warn("Failed to unmarshal user during Google ID scan", slog.String("key", string(item.Key())), slog.Any("error", err))
				continue
			}
			if user.GoogleID == googleID {
				foundUser = &user
				return nil // Found, stop iteration
			}
		}
		return nil // Finished iteration without finding
	})

	if err != nil {
		logger.Error("Error during user scan by Google ID", slog.Any("error", err))
		return nil, err
	}
	if foundUser == nil {
		return nil, errNotFound
	}
	return foundUser, nil
}

func deleteUser(uuid string) error {
	key := userPrefix + uuid
	dbID := getShardIndex(key)
	return deleteItem(stores[dbID], []byte(userPrefix+uuid))
}

func listUsers() ([]User, error) {
	users := make([]User, 0)
	for i := range numShards {
		rawUsers, err := listItemsRaw(stores[i], []byte(userPrefix))
		if err != nil {
			return nil, fmt.Errorf("failed to list raw users: %w", err)
		}
		for _, raw := range rawUsers {
			var user User
			if err := json.Unmarshal(raw, &user); err != nil {
				logger.Warn("Failed to unmarshal user during list", slog.Any("error", err))
				continue
			}
			users = append(users, user)
		}

	}
	return users, nil
}

// --- Username Index ---

func setUsernameIndex(username, userUUID string) error {
	key := []byte(indexPrefix + username)
	value := []byte(userUUID)
	dbID := getShardIndex(string(key))

	err := indexes[dbID].Update(func(txn *badger.Txn) error {
		// Optionally: Check if username is already taken by a *different* user UUID
		// item, err := txn.Get(key)
		// if err == nil {
		//     existingUUID, _ := item.ValueCopy(nil)
		//     if string(existingUUID) != userUUID {
		//         return errors.New("username already taken")
		//     }
		// } else if err != badger.ErrKeyNotFound {
		//     return err
		// }
		return txn.Set(key, value)
	})
	return err
}

func getUserUUIDByUsername(username string) (string, error) {
	key := []byte(indexPrefix + username)
	dbID := getShardIndex(string(key))
	var userUUID string
	err := indexes[dbID].View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err // Handles ErrKeyNotFound
		}
		uuidBytes, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		userUUID = string(uuidBytes)
		return nil
	})
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return "", fmt.Errorf("username %s not found: %w", username, errNotFound)
		}
		return "", err
	}
	return userUUID, nil
}

func deleteUsernameIndex(username string) error {
	key := []byte(indexPrefix + username)
	dbID := getShardIndex(string(key))
	return deleteItem(indexes[dbID], key)
}

// --- Group Specific ---

func saveGroup(group *Group) error {
	key := groupPrefix + group.UUID
	dbID := getShardIndex(key)
	return createItem(stores[dbID], []byte(key), group)
}

func getGroup(uuid string) (*Group, error) {
	var group Group
	key := groupPrefix + uuid
	dbID := getShardIndex(key)
	err := getItem(stores[dbID], []byte(key), &group)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("group %s not found: %w", uuid, errNotFound)
		}
		return nil, err
	}
	return &group, nil
}

func deleteGroup(uuid string) error {
	key := groupPrefix + uuid
	dbID := getShardIndex(key)
	return deleteItem(stores[dbID], []byte(key))
}

func listGroups() ([]Group, error) {
	groups := make([]Group, 0)
	for i := range numShards {
		rawGroups, err := listItemsRaw(stores[i], []byte(groupPrefix))
		if err != nil {
			return nil, fmt.Errorf("failed to list raw groups: %w", err)
		}
		for _, raw := range rawGroups {
			var group Group
			if err := json.Unmarshal(raw, &group); err != nil {
				logger.Warn("Failed to unmarshal group during list", slog.Any("error", err))
				continue
			}
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// --- Server Specific ---

func saveServer(server *Server) error {
	key := serverPrefix + server.UUID
	dbID := getShardIndex(key)
	return createItem(stores[dbID], []byte(key), server)
}

func getServer(uuid string) (*Server, error) {
	var server Server
	key := serverPrefix + uuid
	dbID := getShardIndex(key)
	err := getItem(stores[dbID], []byte(key), &server)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("server %s not found: %w", uuid, errNotFound)
		}
		return nil, err
	}
	return &server, nil
}

func deleteServer(uuid string) error {
	key := serverPrefix + uuid
	dbID := getShardIndex(key)
	return deleteItem(stores[dbID], []byte(key))
}

func listServers() ([]Server, error) {
	servers := make([]Server, 0)
	for i := range numShards {
		rawServers, err := listItemsRaw(stores[i], []byte(serverPrefix))
		if err != nil {
			return nil, fmt.Errorf("failed to list raw servers: %w", err)
		}
		for _, raw := range rawServers {
			var server Server
			if err := json.Unmarshal(raw, &server); err != nil {
				logger.Warn("Failed to unmarshal server during list", slog.Any("error", err))
				continue
			}
			servers = append(servers, server)
		}
	}
	return servers, nil
}

// --- Token Specific ---

func saveToken(token *AuthToken) error {
	key := tokenPrefix + token.TokenUUID
	dbID := getShardIndex(key)
	return createItem(stores[dbID], []byte(key), token)
}

func getToken(tokenUUID string) (*AuthToken, error) {
	var token AuthToken
	key := tokenPrefix + tokenUUID
	dbID := getShardIndex(key)

	// Add TTL check conceptually here if needed, though Badger doesn't directly expire based on struct field
	// Tokens should probably have an expiry set on the key using badger's SetEntry or checked manually
	err := getItem(stores[dbID], []byte(key), &token)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("token %s not found: %w", tokenUUID, errNotFound)
		}
		return nil, err
	}

	// Optional: Manual expiry check
	// if time.Since(token.CreatedAt) > tokenLifetime {
	//     deleteToken(tokenUUID) // Clean up expired token
	//     return nil, fmt.Errorf("token %s expired: %w", tokenUUID, ErrUnauthorized)
	// }

	return &token, nil
}

func deleteToken(tokenUUID string) error {
	key := tokenPrefix + tokenUUID
	dbID := getShardIndex(key)
	return deleteItem(stores[dbID], []byte(key))
}

func deleteAllUserTokens(userUUID string) error {
	tokensToDelete := []string{}

	// We need to check all shards for tokens related to this user
	for i := range numShards {
		err := stores[i].View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = []byte(tokenPrefix)
			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek([]byte(tokenPrefix)); it.ValidForPrefix([]byte(tokenPrefix)); it.Next() {
				item := it.Item()
				var token AuthToken
				err := item.Value(func(val []byte) error {
					return json.Unmarshal(val, &token)
				})
				if err == nil && token.UserUUID == userUUID {
					tokensToDelete = append(tokensToDelete, token.TokenUUID)
				} else if err != nil {
					logger.Warn("Failed to unmarshal token during user token cleanup scan", slog.String("key", string(item.Key())), slog.Any("error", err))
				}
			}
			return nil
		})
		if err != nil {
			logger.Error("Error scanning tokens for deletion", slog.String("userUUID", userUUID), slog.Any("error", err))
			return err // Error during the scan itself
		}
	}

	// Perform deletions by calculating the appropriate shard for each token
	if len(tokensToDelete) > 0 {
		for _, tokenUUID := range tokensToDelete {
			key := tokenPrefix + tokenUUID
			dbID := getShardIndex(key)

			err := stores[dbID].Update(func(txn *badger.Txn) error {
				if err := txn.Delete([]byte(key)); err != nil {
					// Log but attempt to continue deleting others
					logger.Error("Error deleting specific token during batch cleanup", slog.String("tokenUUID", tokenUUID), slog.Any("error", err))
				}
				return nil
			})

			if err != nil {
				logger.Error("Error during token deletion transaction", slog.String("tokenUUID", tokenUUID), slog.Any("error", err))
				// Continue with other tokens even if this one failed
			}
		}
	}

	return nil
}
