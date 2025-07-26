package argon

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Argon struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func NewDefault() *Argon {
	return &Argon{
		Memory:      20 * 1024, // 20 MiB
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func (a *Argon) Key(password string, skipSalt bool) (key []byte, err error) {
	salt := make([]byte, a.SaltLength)
	if !skipSalt {
		if _, err := rand.Read(salt); err != nil {
			return nil, err
		}
	}
	key = argon2.IDKey([]byte(password), salt, a.Iterations, a.Memory, a.Parallelism, a.KeyLength)
	return
}

func (a *Argon) Hash(data string) (encodedHash string, err error) {
	salt := make([]byte, a.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(data), salt, a.Iterations, a.Memory, a.Parallelism, a.KeyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, a.Memory, a.Iterations, a.Parallelism, b64Salt, b64Hash)
	return encodedHash, nil
}

func (a *Argon) Compare(data, encodedHash string) (match bool, err error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	if version != argon2.Version {
		return false, errors.New("incompatible version")
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	keyLength := uint32(len(hash))

	// Compute hash for input password
	computedHash := argon2.IDKey([]byte(data), salt, iterations, memory, parallelism, keyLength)

	// Constant-time compare
	return subtle.ConstantTimeCompare(hash, computedHash) == 1, nil
}

func GenerateUserFolderHash(userID string) (key []byte, err error) {
	a := &Argon{
		Memory:      20 * 1024, // 20 MiB
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	key, err = a.Key(userID, true)
	if err != nil {
		return nil, err
	}

	return
}

func GetKeyFromLocalInfo(extraParams ...any) (key []byte, err error) {
	preHash := ""
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	wdl := strings.Split(wd, string(os.PathSeparator))
	if len(wdl) < 1 {
		return nil, fmt.Errorf("could not find pwd")
	}
	preHash += wdl[len(wdl)-1]

	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}
	edl := strings.Split(ex, string(os.PathSeparator))
	if len(edl) < 1 {
		return nil, fmt.Errorf("could not find pwd")
	}
	preHash += edl[len(edl)-1]
	for _, v := range extraParams {
		preHash += fmt.Sprintf("%s", v)
	}

	a := &Argon{
		Memory:      20 * 1024, // 20 MiB
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	key, err = a.Key(preHash, true)
	if err != nil {
		return nil, err
	}

	return
}
