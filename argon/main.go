package argon

import (
	"crypto/rand"
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

func GenerateUserFolderHash(userID string) (key []byte, err error) {
	a := &Argon{
		Memory:      20 * 1024,
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
		Memory:      20 * 1024,
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
