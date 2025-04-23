package crypt

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"os"
	"runtime/debug"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/net/quic"
)

type CurveType int

const (
	P521 CurveType = iota
	X25519
)

type EncType int

const (
	None EncType = iota
	AES128
	AES256
	CHACHA20
)

type SEAL struct {
	Created   time.Time
	CurveType CurveType

	secret1   []byte
	key1      []byte
	AEAD1     cipher.AEAD
	Nonce1    []byte
	Nonce1Len int
	Nonce1U   atomic.Uint64

	secret2   []byte
	key2      []byte
	AEAD2     cipher.AEAD
	Nonce2    []byte
	Nonce2Len int
	Nonce2U   atomic.Uint64

	PrivateKey *ecdh.PrivateKey
	PublicKey  *ecdh.PublicKey

	Type EncType
}

func (S *SEAL) HKDF(keySize int) (err error) {
	hash := sha256.New

	// KYBER IS NOT YET IMPLEMENTED
	// Once kyber is implemented we will assign it's
	// secret key to S.key2 and add it to the HKDF.
	// h := hkdf.New(hash, append(S.secret1, S.secret2...), nil, nil)

	h := hkdf.New(hash, S.secret1, nil, nil)
	var n int
	S.key1 = make([]byte, keySize)
	n, err = io.ReadFull(h, S.key1)
	if err != nil {
		return
	}
	if n != keySize {
		return errors.New("could not read keySize finalKey 1")
	}

	S.key2 = make([]byte, keySize)
	n, err = io.ReadFull(h, S.key2)
	if err != nil {
		return
	}
	if n != keySize {
		return errors.New("could not read keySize for finalKey2")
	}

	return
}

func (S *SEAL) CreateAEAD() (err error) {
	err = S.GetSecret1()
	if err != nil {
		return
	}
	err = S.GetSecret2()
	if err != nil {
		return
	}

	if S.Type == AES128 {
		err = S.HKDF(16)
	} else {
		err = S.HKDF(32)
	}
	if err != nil {
		return
	}

	if S.Type == None {
		return errors.New("no encryption is not supported")
	} else if S.Type == AES256 || S.Type == AES128 {

		if S.Type == AES128 {
			S.key1 = S.key1[:16]
			S.key2 = S.key2[:16]
		}

		CB1, CB1Err := aes.NewCipher(S.key1)
		if CB1Err != nil {
			err = CB1Err
			return
		}

		CB2, CB2Err := aes.NewCipher(S.key2)
		if CB2Err != nil {
			err = CB2Err
			return
		}

		S.AEAD1, err = cipher.NewGCM(CB1)
		if err != nil {
			return
		}
		S.Nonce1 = make([]byte, S.AEAD1.NonceSize())
		S.Nonce1Len = S.AEAD1.NonceSize()

		S.AEAD2, err = cipher.NewGCM(CB2)
		if err != nil {
			return
		}
		S.Nonce2 = make([]byte, S.AEAD2.NonceSize())
		S.Nonce2Len = S.AEAD2.NonceSize()

	} else if S.Type == CHACHA20 {

		S.AEAD1, err = chacha20poly1305.NewX(S.key1)
		if err != nil {
			return
		}
		S.Nonce1 = make([]byte, S.AEAD1.NonceSize())
		S.Nonce1Len = S.AEAD1.NonceSize()

		S.AEAD2, err = chacha20poly1305.NewX(S.key2)
		if err != nil {
			return
		}
		S.Nonce2 = make([]byte, S.AEAD2.NonceSize())
		S.Nonce2Len = S.AEAD2.NonceSize()
	} else {
		return errors.New("no encryption is not supported")
	}

	return
}

func (S *SEAL) Encrypt1(data []byte) []byte {
	return S.AEAD1.Seal(nil, S.Nonce1, data, nil)
}

func (S *SEAL) Encrypt2(data []byte, staging []byte) []byte {
	return S.AEAD2.Seal(nil, S.Nonce2, data, nil)
}

func (S *SEAL) Seal1(data []byte, index []byte) (out []byte) {
	n := make([]byte, S.Nonce1Len)
	binary.BigEndian.PutUint64(n, S.Nonce1U.Add(1))
	out = []byte{index[0], index[1], n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7]}
	out = S.AEAD1.Seal(out, n, data, index)
	return
}

func (S *SEAL) Seal2(data []byte, index []byte) (out []byte) {
	n := make([]byte, S.Nonce2Len)
	binary.BigEndian.PutUint64(n, S.Nonce2U.Add(1))
	out = []byte{index[0], index[1], n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7]}
	out = S.AEAD2.Seal(out, n, data, index)
	return
}

func (S *SEAL) Open1(data []byte, nonce []byte, staging []byte, index []byte) (decrypted []byte, err error) {
	n := make([]byte, S.Nonce1Len)
	n[0] = nonce[0]
	n[1] = nonce[1]
	n[2] = nonce[2]
	n[3] = nonce[3]
	n[4] = nonce[4]
	n[5] = nonce[5]
	n[6] = nonce[6]
	n[7] = nonce[7]
	decrypted, err = S.AEAD1.Open(
		staging,
		n,
		data,
		index,
	)
	return
}

func (S *SEAL) Open2(data []byte, nonce []byte, staging []byte, index []byte) (decrypted []byte, err error) {
	n := make([]byte, S.Nonce2Len)
	n[0] = nonce[0]
	n[1] = nonce[1]
	n[2] = nonce[2]
	n[3] = nonce[3]
	n[4] = nonce[4]
	n[5] = nonce[5]
	n[6] = nonce[6]
	n[7] = nonce[7]
	decrypted, err = S.AEAD2.Open(
		staging,
		n,
		data,
		index,
	)
	return
}

func (S *SEAL) GetSecret2() (err error) {
	S.secret2 = make([]byte, 32)
	return
}

func (S *SEAL) GetSecret1() (err error) {
	var nk []byte
	nk, err = S.PrivateKey.ECDH(S.PublicKey)
	sk := sha256.Sum256(nk)
	S.secret1 = sk[:]
	return
}

func (S *SEAL) PublicKeyFromBytes(publicKey []byte) (err error) {
	if S.CurveType == X25519 {
		S.PublicKey, err = ecdh.X25519().NewPublicKey(publicKey)
	} else {
		S.PublicKey, err = ecdh.P521().NewPublicKey(publicKey)
	}
	if err != nil {
		return
	}
	return
}

func (S *SEAL) NewPrivateKey() (PK *ecdh.PrivateKey, err error) {
	if S.CurveType == X25519 {
		PK, err = ecdh.X25519().GenerateKey(rand.Reader)
	} else {
		PK, err = ecdh.P521().GenerateKey(rand.Reader)
	}
	if err != nil {
		return
	}
	return
}

func (S *SEAL) NewPublicKeyFromBytes(b []byte) (PK *ecdh.PublicKey, err error) {
	if S.CurveType == X25519 {
		PK, err = ecdh.X25519().NewPublicKey(b)
	} else {
		PK, err = ecdh.P521().NewPublicKey(b)
	}
	if err != nil {
		return
	}
	return
}

type SocketWrapper struct {
	LocalPK  *ecdh.PrivateKey
	RemotePK *ecdh.PublicKey
	SEAL     *SEAL

	HStream *quic.Stream
	HConn   net.Conn
}

func (T *SocketWrapper) SetHandshakeStream(s *quic.Stream) {
	T.HStream = s
}

func (T *SocketWrapper) SetHandshakeConn(c net.Conn) {
	T.HConn = c
}

func NewEncryptionHandler(
	encryptionType EncType,
	curveType CurveType,
) (T *SocketWrapper, err error) {
	T = new(SocketWrapper)
	T.SEAL = new(SEAL)
	T.SEAL.CurveType = curveType
	T.SEAL.Created = time.Now()
	T.SEAL.Type = encryptionType
	T.SEAL.PrivateKey, err = T.SEAL.NewPrivateKey()
	T.SEAL.Nonce1U.Store(0)
	T.SEAL.Nonce2U.Store(0)
	return
}
func (T *SocketWrapper) GetPublicKey() (key []byte) {
	return T.SEAL.PrivateKey.PublicKey().Bytes()
}

// func (T *SocketWrapper) InitHandshake() (err error) {
// 	err = T.SendPublicKey()
// 	if err != nil {
// 		return
// 	}

// 	err = T.ReceivePublicKey()
// 	if err != nil {
// 		return
// 	}

// 	err = T.SEAL.CreateAEAD()
// 	return
// }

// func (T *SocketWrapper) ReceiveHandshake() (err error) {
// 	err = T.ReceivePublicKey()
// 	if err != nil {
// 		return
// 	}

// 	err = T.SendPublicKey()
// 	if err != nil {
// 		return
// 	}

// 	err = T.SEAL.CreateAEAD()
// 	return
// }

func (T *SocketWrapper) SendPublicKey() (err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	if T.HConn != nil {
		_, err = T.HConn.Write(T.SEAL.PrivateKey.PublicKey().Bytes())
	} else {
		_, err = T.HStream.Write(T.SEAL.PrivateKey.PublicKey().Bytes())
		T.HStream.Flush()
	}

	return err
}

func (T *SocketWrapper) ReceivePublicKey() (err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	PublicKey := make([]byte, math.MaxUint16)
	var n int
	var re error
	if T.HConn != nil {
		n, re = T.HConn.Read(PublicKey)
	} else {
		n, re = T.HStream.Read(PublicKey)
	}
	if re != nil {
		return re
	}

	T.SEAL.PublicKey, err = T.SEAL.NewPublicKeyFromBytes(PublicKey[:n])
	return
}

type SignedConnectRequest struct {
	Signature []byte
	Payload   []byte
}

func SignPayload(obj interface{}, privateKey *rsa.PrivateKey) (w *SignedConnectRequest, err error) {
	var encB []byte
	encB, err = json.Marshal(obj)
	if err != nil {
		return
	}

	w = new(SignedConnectRequest)
	hash := sha256.New()
	hash.Write(encB)

	w.Payload = encB
	w.Signature, err = rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash.Sum(nil))
	if err != nil {
		return nil, err
	}

	return
}

func ValidateSignature(wrapper []byte, publicKey *rsa.PublicKey) (w SignedConnectRequest, err error) {
	err = json.Unmarshal(wrapper, &w)
	if err != nil {
		return
	}

	hash := sha256.New()
	hash.Write(w.Payload)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash.Sum(nil), w.Signature); err == nil {
		return w, nil
	}

	err = errors.New("Signatures do not match")
	return
}

func LoadPrivateKey(path string) (privateKey *rsa.PrivateKey, err error) {
	privateKeyPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	privateKeyBlock, _ := pem.Decode(privateKeyPEM)

	var anyKey any
	anyKey, err = x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	var ok bool
	privateKey, ok = anyKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private signing key was not rsa.PrivateKey")
	}

	return
}
