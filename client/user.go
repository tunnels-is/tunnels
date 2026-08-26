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

	"golang.org/x/crypto/argon2"
)

const (
	accountCryptoVersion byte = 1
	accountSaltLen            = 16

	accountKDFTime    = 2
	accountKDFMemory  = 32 * 1024
	accountKDFThreads = 1
	accountKDFKeyLen  = 32
)

func deriveAccountFileKey(folderHash string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(folderHash),
		salt,
		accountKDFTime,
		accountKDFMemory,
		accountKDFThreads,
		accountKDFKeyLen,
	)
}

func encryptAccountBlob(plaintext []byte, folderHash string) ([]byte, error) {
	if !isHexString(folderHash) {
		return nil, fmt.Errorf("invalid account hash")
	}
	salt := make([]byte, accountSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := deriveAccountFileKey(folderHash, salt)

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
	ct := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 1+accountSaltLen+len(nonce)+len(ct))
	out[0] = accountCryptoVersion
	copy(out[1:], salt)
	copy(out[1+accountSaltLen:], nonce)
	copy(out[1+accountSaltLen+len(nonce):], ct)
	return out, nil
}

func decryptAccountBlob(blob []byte, folderHash string) ([]byte, error) {
	if !isHexString(folderHash) {
		return nil, fmt.Errorf("invalid account hash")
	}
	if len(blob) < 1+accountSaltLen+12 {
		return nil, errors.New("ciphertext too short")
	}
	if blob[0] != accountCryptoVersion {
		return nil, fmt.Errorf("unsupported account crypto version %d", blob[0])
	}
	salt := blob[1 : 1+accountSaltLen]
	key := deriveAccountFileKey(folderHash, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	rest := blob[1+accountSaltLen:]
	if len(rest) < nonceSize+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := rest[:nonceSize], rest[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func delUser(hash string) (err error) {
	if !isHexString(hash) {
		return fmt.Errorf("invalid user hash")
	}
	s := STATE.Load()
	DEBUG("removing account: ", hash)
	dir := accountDir(hash)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if s.ActiveAccountHash == hash {
		s.ActiveAccountHash = ""
		s.TunnelsPath = ""
		s.DevicesPath = ""
		STATE.Store(s)
		clearTunnelMetaMap()
	}
	return nil
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
	hash, err := userIDToAccountHash(u.ID)
	if err != nil {
		return err
	}
	u.SaveFileHash = hash

	if err := ensureAccountDirs(hash); err != nil {
		return err
	}

	ub, err := json.Marshal(u)
	if err != nil {
		return err
	}

	encrypted, err := encryptAccountBlob(ub, hash)
	if err != nil {
		return err
	}

	path := accountUserFile(hash)
	DEBUG("Saving user:", hash)
	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		return err
	}

	if actErr := activateAccountByHash(hash); actErr != nil {
		ERROR("unable to activate account after save:", actErr)
	}
	return nil
}

func getUsers() (ul []*User, err error) {
	ul = make([]*User, 0)
	s := STATE.Load()

	entries, err := os.ReadDir(s.AccountsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ul, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if !e.IsDir() || !isHexString(e.Name()) {
			continue
		}
		hash := e.Name()
		path := accountUserFile(hash)
		fb, er := os.ReadFile(path)
		if er != nil {
			if !errors.Is(er, os.ErrNotExist) {
				ERROR("unable to read user file:", path, er)
			}
			continue
		}
		warnIfInsecureSecretFile(path)
		DEBUG("loading user:", hash)
		decrypted, er := decryptAccountBlob(fb, hash)
		if er != nil {
			ERROR("unable to decrypt user file:", hash, " err:", er)
			continue
		}
		if len(decrypted) == 0 {
			continue
		}
		u := new(User)
		if er = json.Unmarshal(decrypted, u); er != nil {
			ERROR("unable to decode user file:", hash)
			continue
		}
		u.SaveFileHash = hash
		ul = append(ul, u)
	}

	return ul, nil
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
