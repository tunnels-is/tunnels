package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func delUser() (err error) {
	s := STATE.Load()
	_ = os.Remove(s.BasePath + "enc")
	return
}

func saveUser(u *User) (err error) {
	ul, err := loadUsers()
	if ul == nil {
		ul = new(SavedUserList)
	}

	hasUser := false
	for i, v := range ul.Users {
		if v.ID == u.ID {
			ul.Users[i] = u
			hasUser = true
		}
	}
	if !hasUser {
		ul.Users = append(ul.Users, u)
	}

	ub, err := json.Marshal(ul)
	if err != nil {
		return err
	}
	encryptged, err := Encrypt(ub, []byte("01234567890123456789012345678900"))
	if err != nil {
		return err
	}

	s := STATE.Load()
	_ = RemoveFile(s.BasePath + "enc")
	f, err := CreateFile(s.BasePath + "enc")
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

func loadUsers() (ul *SavedUserList, err error) {
	ul = new(SavedUserList)

	s := STATE.Load()
	ub, err := os.ReadFile(s.BasePath + "enc")
	if err != nil {
		return nil, err
	}
	if len(ub) == 0 {
		return nil, fmt.Errorf("no user found")
	}

	data := ub[aes.BlockSize:]
	iv := ub[:aes.BlockSize]

	encrypted, err := Decrypt(data, iv, []byte("01234567890123456789012345678900"))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(encrypted, ul)
	if err != nil {
		return nil, err
	}
	return
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
