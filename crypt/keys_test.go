package crypt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func TestVerifySignatureRoundTrip(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello world")
	sig, err := SignData(data, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(data, sig, &priv.PublicKey); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifySignature([]byte("tampered"), sig, &priv.PublicKey); err == nil {
		t.Fatal("tampered data must not verify")
	}
}

// VerifySignature must fail CLOSED when handed a key type it cannot use, rather
// than returning nil (which would treat any signature as valid).
func TestVerifySignatureFailsClosedOnBadKeyType(t *testing.T) {
	if err := VerifySignature([]byte("data"), []byte("sig"), nil); err == nil {
		t.Fatal("nil key must not verify")
	}
	if err := VerifySignature([]byte("data"), []byte("sig"), "not-a-key"); err == nil {
		t.Fatal("unrecognized key type must not verify")
	}
}
