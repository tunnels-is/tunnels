package client

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/xlzd/gotp"
)

func GetQRCode(form *TwoFactorConfirm) (*QRCode, error) {
	if form.Email == "" {
		return nil, errors.New("email missing")
	}

	secret := make([]rune, 16)
	for i := range secret {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(totpAlphabet))))
		if err != nil {
			return nil, err
		}
		secret[i] = totpAlphabet[n.Int64()]
	}

	totpSecret := strings.ToUpper(string(secret))
	return &QRCode{
		Value: gotp.NewDefaultTOTP(totpSecret).ProvisioningUri(form.Email, "Tunnels"),
	}, nil
}
