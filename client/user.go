package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/tunnels-is/tunnels/argon"
)

const userKeyFileName = "user.key"

var userKeyMu sync.Mutex

// getUserFileKey returns the AES-256 key protecting saved user files: 32
// random bytes generated once per install and stored 0600 in BasePath. The
// previous scheme derived the key from public filesystem names with a zero
// salt (argon.GetKeyFromLocalInfo), which any local file reader could
// recompute — files encrypted that way are transparently migrated in
// getUsers. This key still lives on the same disk as the user files, so it
// defends against other local users and casual file exfiltration, not
// against an attacker with full access to this account's files.
func getUserFileKey() ([]byte, error) {
	userKeyMu.Lock()
	defer userKeyMu.Unlock()

	s := STATE.Load()
	path := s.BasePath + userKeyFileName
	kb, err := os.ReadFile(path)
	if err == nil {
		key, derr := base64.StdEncoding.DecodeString(string(kb))
		if derr != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid user key file %q — delete it to re-generate (saved logins will be lost)", path)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func delUser(hash string) (err error) {
	s := STATE.Load()
	DEBUG("removing user: ", hash)
	_ = os.Remove(s.UserPath + hash)
	return
}

func saveUser(u *User) (err error) {
	key, err := getUserFileKey()
	if err != nil {
		return err
	}
	userFile, err := argon.GenerateUserFolderHash(u.ID)
	if err != nil {
		return err
	}
	u.SaveFileHash = fmt.Sprintf("%x", userFile)

	ub, err := json.Marshal(u)
	if err != nil {
		return err
	}

	encryptged, err := Encrypt(ub, key)
	if err != nil {
		return err
	}

	s := STATE.Load()
	DEBUG("Saving user:", fmt.Sprintf("%x", userFile))
	f, err := CreateFile(s.UserPath + fmt.Sprintf("%x", userFile))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(encryptged)
	if err != nil {
		return err
	}

	return nil
}

func getUsers() (ul []*User, err error) {
	ul = make([]*User, 0)
	s := STATE.Load()
	key, err := getUserFileKey()
	if err != nil {
		return nil, err
	}

	// Legacy key (public-info derivation, zero salt) — only computed if an
	// old-format file is actually encountered, and only once.
	var legacyKey []byte
	legacyKeyOnce := func() []byte {
		if legacyKey == nil {
			legacyKey, _ = argon.GetKeyFromLocalInfo()
		}
		return legacyKey
	}

	err = filepath.WalkDir(s.UserPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if err != nil {
			ERROR("unable to walk path", err)
			return nil
		}
		base := filepath.Base(path)
		DEBUG("loading user:", base)
		fb, er := os.ReadFile(path)
		if er != nil {
			ERROR("unable to read user file:", er)
			return nil
		}
		migrate := false
		decrypted, er := Decrypt(fb, key)
		if er != nil {
			if lk := legacyKeyOnce(); lk != nil {
				decrypted, er = Decrypt(fb, lk)
				migrate = er == nil
			}
			if er != nil {
				// Undecryptable with either key: skip this file instead of
				// aborting the walk (one corrupt file must not hide all users).
				ERROR("unable to decrypt user file:", base, " err:", er)
				return nil
			}
		}
		if len(decrypted) == 0 {
			return nil
		}

		u := new(User)
		er = json.Unmarshal(decrypted, u)
		if er != nil {
			ERROR("unable to decode user file (user will be removed):", base)
			_ = os.Remove(path)
			return nil
		}
		if u.SaveFileHash == "" {
			u.SaveFileHash = base
		}

		if migrate {
			if serr := saveUser(u); serr != nil {
				ERROR("unable to migrate user file to new key:", base, " err:", serr)
			} else {
				DEBUG("migrated user file to per-install key:", base)
			}
		}

		ul = append(ul, u)
		return nil
	})

	return ul, err
}

func Decrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

func Encrypt(text, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, text, nil), nil
}
