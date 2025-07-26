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

func delUser(u *User) (err error) {
	s := STATE.Load()
	userFile, err := argon.GenerateUserFolderHash(u.ID)
	if err != nil {
		return err
	}
	_ = os.Remove(s.UserPath + fmt.Sprintf("%x", userFile))
	return
}

func saveUser(u *User) (err error) {
	ub, err := json.Marshal(u)
	if err != nil {
		return err
	}

	key, err := argon.GetKeyFromLocalInfo(version)
	if err != nil {
		return err
	}
	userFile, err := argon.GenerateUserFolderHash(u.ID)
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
	key, err := argon.GetKeyFromLocalInfo(version)
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
		fb, er := os.ReadFile(path)
		if er != nil {
			ERROR("unable to read user file:", er)
		}
		data := fb[aes.BlockSize:]
		iv := fb[:aes.BlockSize]

		DEBUG("loading user:", path)
		decrypted, er := Decrypt(data, iv, key)
		if er != nil {
			return er
		}

		u := new(User)
		er = json.Unmarshal(decrypted, u)
		if er != nil {
			ERROR("unable to decode user file:", er)
		}

		ul = append(ul, u)
		return nil
	})

	return ul, err
}

func getSTREAM(key []byte, iv []byte) (cipher.Stream, []byte, error) {
	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	// Generate a random IV (same size as block size, 16 bytes for AES)
	if iv == nil {
		iv = make([]byte, aes.BlockSize)
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, nil, err
		}
	}

	// Create GCM mode
	ctr := cipher.NewCTR(block, iv)
	if ctr == nil {
		return nil, nil, errors.New("unable to create ctr")
	}

	return ctr, iv, err
}

func Decrypt(text, iv []byte, key []byte) ([]byte, error) {
	stream, _, err := getSTREAM(key, iv)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(text))
	stream.XORKeyStream(out, text)
	return out, nil
}

func Encrypt(text, key []byte) ([]byte, error) {
	stream, iv, err := getSTREAM(key, nil)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(text))
	stream.XORKeyStream(out, text)

	return append(iv, out...), nil
}
