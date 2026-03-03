package main

import (
	"crypto/hmac"
	"encoding/base64"
	"hash"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const (
	noiseConstruction = "Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s"
	noiseIdentifier   = "WireGuard v1 zx2c4 Jason@zx2c4.com"
	msgInitiationSize = 148
)

func noiseHMAC(key, data []byte) []byte {
	mac := hmac.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func noiseHash(inputs ...[]byte) []byte {
	h, _ := blake2s.New256(nil)
	for _, b := range inputs {
		h.Write(b)
	}
	return h.Sum(nil)
}

func noiseKDF1(key, input []byte) []byte {
	extract := noiseHMAC(key, input)
	return noiseHMAC(extract, []byte{0x1})
}

func noiseKDF2(key, input []byte) (t1, t2 []byte) {
	extract := noiseHMAC(key, input)
	t1 = noiseHMAC(extract, []byte{0x1})
	t2 = noiseHMAC(extract, append(t1, 0x2))
	return
}

func tryDecryptInitiator(pkt []byte, serverPriv, serverPub []byte) (pubKeyB64 string, ok bool) {
	if len(pkt) < msgInitiationSize {
		return "", false
	}

	clientEphemeral := pkt[8:40]

	Ci := noiseHash([]byte(noiseConstruction))
	Hi := noiseHash(noiseHash(Ci, []byte(noiseIdentifier)), serverPub)

	Ci = noiseKDF1(Ci, clientEphemeral)
	Hi = noiseHash(Hi, clientEphemeral)

	shared, err := curve25519.X25519(serverPriv, clientEphemeral)
	if err != nil {
		return "", false
	}

	_, k := noiseKDF2(Ci, shared)

	aead, err := chacha20poly1305.New(k)
	if err != nil {
		return "", false
	}
	nonce := make([]byte, aead.NonceSize())
	plaintext, err := aead.Open(nil, nonce, pkt[40:88], Hi)
	if err != nil {

		return "", false
	}

	return base64.StdEncoding.EncodeToString(plaintext), true
}
