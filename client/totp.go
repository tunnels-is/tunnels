package client

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/xlzd/gotp"
)

func GetQRCode(LF *TwoFactorConfirm) (QR *QRCode, err error) {
	if LF.Email == "" {
		return nil, errors.New("email missing")
	}

	b := make([]rune, 16)
	for i := range b {
		n, cerr := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if cerr != nil {
			return nil, cerr
		}
		b[i] = letterRunes[n.Int64()]
	}

	TOTP := strings.ToUpper(string(b))

	authenticatorAppURL := gotp.NewDefaultTOTP(TOTP).ProvisioningUri(LF.Email, "Tunnels")

	QR = new(QRCode)
	QR.Value = authenticatorAppURL

	return QR, nil
}
