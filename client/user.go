package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tunnels-is/tunnels/argon"
)

func delUser(hash string) (err error) {
	s := STATE.Load()
	DEBUG("removing user: ", hash)
	_ = os.Remove(s.UserPath + hash)
	return
}

func saveUser(u *User) (err error) {
	key, err := argon.GetKeyFromLocalInfo()
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
	key, err := argon.GetKeyFromLocalInfo()
	if err != nil {
		return nil, err
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
		}
		decrypted, er := Decrypt(fb, key)
		if er != nil {
			return er
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
