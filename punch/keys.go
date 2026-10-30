package punch

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// DiscoPrivateFor ...
func DiscoPrivateFor(wgPriv [32]byte) [32]byte {
	mac := hmac.New(sha256.New, wgPriv[:])
	mac.Write([]byte("tunnels.is punch disco key v1"))
	sum := mac.Sum(nil)
	var priv [32]byte
	copy(priv[:], sum)
	clamp(&priv)
	return priv
}

// NewPrivate ...
func NewPrivate() ([32]byte, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return priv, err
	}
	clamp(&priv)
	return priv, nil
}

func clamp(priv *[32]byte) {
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
}

// Public ...
func Public(priv [32]byte) ([32]byte, error) {
	var pub [32]byte
	raw, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return pub, err
	}
	copy(pub[:], raw)
	return pub, nil
}

// EncodeKey ...
func EncodeKey(k [32]byte) string {
	return base64.StdEncoding.EncodeToString(k[:])
}

// DecodeKey ...
func DecodeKey(s string) ([32]byte, error) {
	var out [32]byte
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("decode key: %w", err)
	}
	if len(raw) != 32 {
		return out, fmt.Errorf("decode key: got %d bytes, want 32", len(raw))
	}
	copy(out[:], raw)
	return out, nil
}

func sessionKey(priv, peerPub [32]byte) ([32]byte, error) {
	var key [32]byte
	shared, err := curve25519.X25519(priv[:], peerPub[:])
	if err != nil {
		return key, err
	}
	h, err := blake2s.New256(nil)
	if err != nil {
		return key, err
	}
	_, _ = h.Write(shared)
	_, _ = h.Write([]byte("tunnels.is punch v1"))
	copy(key[:], h.Sum(nil))
	return key, nil
}

// Seal ...
func Seal(senderPriv, receiverPub [32]byte, plaintext []byte) ([]byte, error) {
	key, err := sessionKey(senderPriv, receiverPub)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	var nonce [chacha20poly1305.NonceSizeX]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	out := make([]byte, len(nonce), len(nonce)+len(plaintext)+aead.Overhead())
	copy(out, nonce[:])
	return aead.Seal(out, nonce[:], plaintext, nil), nil
}

// Open ...
func Open(receiverPriv, senderPub [32]byte, box []byte) ([]byte, error) {
	if len(box) < chacha20poly1305.NonceSizeX {
		return nil, fmt.Errorf("box too short")
	}
	key, err := sessionKey(receiverPriv, senderPub)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	nonce := box[:chacha20poly1305.NonceSizeX]
	plain, err := aead.Open(nil, nonce, box[chacha20poly1305.NonceSizeX:], nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
