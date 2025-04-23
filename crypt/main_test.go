package crypt

import (
	"crypto/rand"
	"testing"
)

type payload struct {
	Data  []byte
	Index []byte
}

var table = make(map[string]*payload)

func RandomBytes(size int) []byte {
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	return buf
}

func createTable() {
	table["10b"] = &payload{
		Index: RandomBytes(2),
		Data:  RandomBytes(10),
	}
	table["100b"] = &payload{
		Index: RandomBytes(2),
		Data:  RandomBytes(100),
	}
	table["1000b"] = &payload{
		Index: RandomBytes(2),
		Data:  RandomBytes(1000),
	}
	table["10000b"] = &payload{
		Index: RandomBytes(2),
		Data:  RandomBytes(10000),
	}
	table["100000b"] = &payload{
		Index: RandomBytes(2),
		Data:  RandomBytes(100000),
	}
}

func MakeHandlerInvalidPair(et1 EncType, et2 EncType, t *testing.T) (H1 *SocketWrapper, H2 *SocketWrapper) {
	EH1, err := NewEncryptionHandler(et1, X25519)
	if err != nil {
		t.Fatal(err)
	}

	PK := EH1.SEAL.PrivateKey.PublicKey().Bytes()

	EH2, err := NewEncryptionHandler(et2, X25519)
	if err != nil {
		t.Fatal(err)
	}

	EH2.SEAL.PublicKey, err = EH2.SEAL.NewPublicKeyFromBytes(PK)
	if err != nil {
		t.Fatal(err)
	}

	PK2 := EH2.SEAL.PrivateKey.PublicKey().Bytes()

	EH1.SEAL.PublicKey, err = EH1.SEAL.NewPublicKeyFromBytes(PK2)
	if err != nil {
		t.Fatal(err)
	}

	err = EH2.SEAL.CreateAEAD()
	if err != nil {
		t.Fatal(err)
	}

	err = EH1.SEAL.CreateAEAD()
	if err != nil {
		t.Fatal(err)
	}

	return EH1, EH2
}

func MakeHandlerPair(et EncType, t *testing.T) (H1 *SocketWrapper, H2 *SocketWrapper) {
	EH1, err := NewEncryptionHandler(et, X25519)
	if err != nil {
		t.Fatal(err)
	}

	PK := EH1.SEAL.PrivateKey.PublicKey().Bytes()

	EH2, err := NewEncryptionHandler(et, X25519)
	if err != nil {
		t.Fatal(err)
	}

	EH2.SEAL.PublicKey, err = EH2.SEAL.NewPublicKeyFromBytes(PK)
	if err != nil {
		t.Fatal(err)
	}

	PK2 := EH2.SEAL.PrivateKey.PublicKey().Bytes()

	EH1.SEAL.PublicKey, err = EH1.SEAL.NewPublicKeyFromBytes(PK2)
	if err != nil {
		t.Fatal(err)
	}

	err = EH2.SEAL.CreateAEAD()
	if err != nil {
		t.Fatal(err)
	}

	err = EH1.SEAL.CreateAEAD()
	if err != nil {
		t.Fatal(err)
	}

	return EH1, EH2
}

func Test_AES128(t *testing.T) {
	createTable()

	EH1, EH2 := MakeHandlerPair(AES128, t)
	var err error
	staging := make([]byte, 200000)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}
}

func Test_AES256(t *testing.T) {
	createTable()

	EH1, EH2 := MakeHandlerPair(AES256, t)
	var err error
	staging := make([]byte, 200000)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}
}

func Test_ChaCha20(t *testing.T) {
	createTable()

	EH1, EH2 := MakeHandlerPair(CHACHA20, t)
	var err error
	staging := make([]byte, 200000)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err != nil {
			t.Fatalf("Seal1/Open1 failed @ %s with error: %s", i, err)
		}
	}
}

func Test_Invalid(t *testing.T) {
	createTable()

	var err error
	staging := make([]byte, 200000)

	EH1, EH2 := MakeHandlerInvalidPair(CHACHA20, AES256, t)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 did not fail @ %s", i)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 failed @ %s", i)
		}
	}

	EH1, EH2 = MakeHandlerInvalidPair(CHACHA20, AES128, t)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 did not fail @ %s", i)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 failed @ %s", i)
		}
	}

	EH1, EH2 = MakeHandlerInvalidPair(AES256, AES128, t)
	for i, v := range table {
		x := EH1.SEAL.Seal1(v.Data, v.Index)
		_, err = EH2.SEAL.Open1(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 did not fail @ %s", i)
		}
	}

	for i, v := range table {
		x := EH2.SEAL.Seal2(v.Data, v.Index)
		_, err = EH1.SEAL.Open2(x[10:], x[2:10], staging[:0], x[0:2])
		if err == nil {
			t.Fatalf("Seal1/Open1 failed @ %s", i)
		}
	}
}
