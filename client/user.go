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

func getUserFileKey() ([]byte, error) {
	userKeyMu.Lock()
	defer userKeyMu.Unlock()

	s := STATE.Load()
	path := s.BasePath + userKeyFileName
	kb, err := os.ReadFile(path)
	if err == nil && len(kb) != 0 {

		if info, statErr := os.Stat(path); statErr == nil {
			if verr := validateUserKeyFile(info); verr != nil {
				return nil, fmt.Errorf("user key file %q: %w — delete it to re-generate (saved logins will be lost)", path, verr)
			}
		}
		key, derr := base64.StdEncoding.DecodeString(string(kb))
		if derr != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid user key file %q — delete it to re-generate (saved logins will be lost)", path)
		}
		return key, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if _, statErr := os.Stat(path); statErr == nil {
		if rmErr := os.Remove(path); rmErr != nil {
			return nil, fmt.Errorf("remove empty user key file %q: %w", path, rmErr)
		}
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create user key file %q: %w", path, err)
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(key)); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func delUser(hash string) (err error) {

	if !isHexString(hash) {
		return fmt.Errorf("invalid user hash")
	}
	s := STATE.Load()
	DEBUG("removing user: ", hash)
	_ = os.Remove(s.UserPath + hash)
	return
}

func isHexString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
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
