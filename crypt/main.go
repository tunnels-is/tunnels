package crypt

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/net/quic"
)

type EncType int

const (
	None EncType = iota
	AES128
	AES256
	CHACHA20
)

type SEAL struct {
	Created time.Time

	key1      []byte
	AEAD1     cipher.AEAD
	Nonce1    []byte
	Nonce1Len int
	Nonce1U   atomic.Uint64

	key2      []byte
	AEAD2     cipher.AEAD
	Nonce2    []byte
	Nonce2Len int
	Nonce2U   atomic.Uint64

	Mlkem1024Decap     *mlkem.DecapsulationKey1024
	Mlkem1024Encap     *mlkem.EncapsulationKey1024
	Mlkem1024PeerEncap *mlkem.EncapsulationKey1024
	Mlkem1024Cipher    []byte

	X25519Priv    *ecdh.PrivateKey
	X25519Pub     *ecdh.PublicKey
	X25519PeerPub *ecdh.PublicKey

	Type EncType
}

func (S *SEAL) CleanPostSecretGeneration() {
	S.Mlkem1024Cipher = nil
	S.Mlkem1024Decap = nil
	S.Mlkem1024Encap = nil
	S.Mlkem1024PeerEncap = nil

	S.X25519PeerPub = nil
	S.X25519Pub = nil
	S.X25519Priv = nil

	S.key1 = nil
	S.key2 = nil
}

func (S *SEAL) HKDF(keySize int, sharedSecret []byte) (err error) {
	h := hkdf.New(sha512.New, sharedSecret, nil, nil)
	var n int
	S.key1 = make([]byte, keySize)
	n, err = io.ReadFull(h, S.key1)
	if err != nil {
		return
	}
	if n != keySize {
		return errors.New("could not read keySize finalKey 1")
	}

	// Key2 might be used later to store a separate shared secret if we want to encrypt each direction with a different key.
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

func (S *SEAL) CreateAEAD(sharedSecret []byte) (err error) {
	if S.Type == AES128 {
		err = S.HKDF(16, sharedSecret)
	} else {
		err = S.HKDF(32, sharedSecret)
	}
	if err != nil {
		return
	}

	switch S.Type {
	case None:
		return errors.New("no encryption is not supported")
	case AES256, AES128:

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

	case CHACHA20:

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
	default:
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
) (T *SocketWrapper) {
	T = new(SocketWrapper)
	T.SEAL = new(SEAL)
	T.SEAL.Created = time.Now()
	T.SEAL.Type = encryptionType
	T.SEAL.Nonce1U.Store(0)
	T.SEAL.Nonce2U.Store(0)
	return
}

func (T *SocketWrapper) InitializeClient() (err error) {
	T.SEAL.X25519Priv, err = ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	T.SEAL.X25519Pub = T.SEAL.X25519Priv.PublicKey()
	T.SEAL.Mlkem1024Decap, err = mlkem.GenerateKey1024()
	if err != nil {
		return err
	}
	T.SEAL.Mlkem1024Encap = T.SEAL.Mlkem1024Decap.EncapsulationKey()
	return nil
}

func (T *SocketWrapper) FinalizeClient(X25519PeerPub []byte, Mlkem1024Cipher []byte) (err error) {
	T.SEAL.X25519PeerPub, err = ecdh.X25519().NewPublicKey(X25519PeerPub)
	if err != nil {
		return
	}
	nk, err := T.SEAL.X25519Priv.ECDH(T.SEAL.X25519PeerPub)
	if err != nil {
		return err
	}
	ss1 := sha256.Sum256(nk)
	s1 := ss1[:]

	s2, err := T.SEAL.Mlkem1024Decap.Decapsulate(Mlkem1024Cipher)
	if err != nil {
		return err
	}

	fss := make([]byte, 0)
	fss = append(fss, s1...)
	fss = append(fss, s2...)
	fss = append(fss, T.SEAL.X25519Pub.Bytes()...)
	fss = append(fss, T.SEAL.X25519PeerPub.Bytes()...)

	return T.SEAL.CreateAEAD(fss)
}

func (T *SocketWrapper) InitializeServer(X25519PeerPub []byte, Mlkem1024Encap []byte) (err error) {
	T.SEAL.X25519Priv, err = ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	T.SEAL.X25519Pub = T.SEAL.X25519Priv.PublicKey()

	T.SEAL.X25519PeerPub, err = ecdh.X25519().NewPublicKey(X25519PeerPub)
	if err != nil {
		return
	}
	T.SEAL.Mlkem1024Encap, err = mlkem.NewEncapsulationKey1024(Mlkem1024Encap)
	if err != nil {
		return
	}
	return
}

func (T *SocketWrapper) FinalizeServer() (err error) {
	nk, err := T.SEAL.X25519Priv.ECDH(T.SEAL.X25519PeerPub)
	if err != nil {
		return err
	}
	ss1 := sha256.Sum256(nk)
	s1 := ss1[:]

	s2, cipherText := T.SEAL.Mlkem1024Encap.Encapsulate()
	T.SEAL.Mlkem1024Cipher = cipherText

	fss := make([]byte, 0)
	fss = append(fss, s1...)
	fss = append(fss, s2...)
	fss = append(fss, T.SEAL.X25519PeerPub.Bytes()...)
	fss = append(fss, T.SEAL.X25519Pub.Bytes()...)

	return T.SEAL.CreateAEAD(fss)
}

type SignedConnectRequest struct {
	Signature []byte
	Payload   []byte
}

func SignPayload(obj any, privateKey *rsa.PrivateKey) (w *SignedConnectRequest, err error) {
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
