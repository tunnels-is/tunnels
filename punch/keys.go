package punch

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
)

// DiscoPrivateFor ...
func DiscoPrivateFor(wgPriv [32]byte) [32]byte {
	mac := hmac.New(sha256.New, wgPriv[:])
	mac.Write([]byte("tunnels.is punch disco key"))
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

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func wipe32(b *[32]byte) {
	for i := range b {
		b[i] = 0
	}
}

func kdf(shared, sender, receiver []byte, label string) [32]byte {
	var key [32]byte
	h, err := blake2s.New256(nil)
	if err != nil {
		return key
	}
	_, _ = h.Write(shared)
	_, _ = h.Write([]byte(label))
	_, _ = h.Write(sender)
	_, _ = h.Write(receiver)
	copy(key[:], h.Sum(nil))
	return key
}

const (
	labelDirect = "tunnels.is punch direct"
	labelEE     = "tunnels.is punch ee"
	labelTag    = "tunnels.is punch tag"
	labelHP     = "tunnels.is punch hp"
	labelBind   = "tunnels.is punch bind"

	labelSessTag = "tunnels.is punch session tag"
	labelSessHP  = "tunnels.is punch session hp"
)

// pairKeys are everything two disco keys derive for direct packets. The
// send/recv fields are per direction: our send is the peer's recv.
type pairKeys struct {
	send, recv       [32]byte
	tagSend, tagRecv [32]byte
	hpSend, hpRecv   [32]byte
	bind             [32]byte
}

func (k *pairKeys) wipe() {
	wipe32(&k.send)
	wipe32(&k.recv)
	wipe32(&k.tagSend)
	wipe32(&k.tagRecv)
	wipe32(&k.hpSend)
	wipe32(&k.hpRecv)
	wipe32(&k.bind)
}

func derivePair(priv, peerPub [32]byte) (pairKeys, error) {
	var k pairKeys
	ourPub, err := Public(priv)
	if err != nil {
		return k, err
	}
	shared, err := curve25519.X25519(priv[:], peerPub[:])
	if err != nil {
		return k, err
	}
	defer wipe(shared)
	k.send = kdf(shared, ourPub[:], peerPub[:], labelDirect)
	k.recv = kdf(shared, peerPub[:], ourPub[:], labelDirect)
	k.tagSend = kdf(shared, ourPub[:], peerPub[:], labelTag)
	k.tagRecv = kdf(shared, peerPub[:], ourPub[:], labelTag)
	k.hpSend = kdf(shared, ourPub[:], peerPub[:], labelHP)
	k.hpRecv = kdf(shared, peerPub[:], ourPub[:], labelHP)
	lo, hi := ourPub, peerPub
	if string(lo[:]) > string(hi[:]) {
		lo, hi = hi, lo
	}
	k.bind = kdf(shared, lo[:], hi[:], labelBind)
	return k, nil
}

// sessKeys are the per-session keys derived from the ephemeral-ephemeral
// DH. Once both sides hold them, AEAD keys, receive tags and the eph mask all
// come from here, so traffic after the first exchange stays unreadable and
// unlinkable even if both disco privates leak later.
type sessKeys struct {
	send, recv       [32]byte
	tagSend, tagRecv [32]byte
	hpSend, hpRecv   [32]byte
}

func (k *sessKeys) wipe() {
	wipe32(&k.send)
	wipe32(&k.recv)
	wipe32(&k.tagSend)
	wipe32(&k.tagRecv)
	wipe32(&k.hpSend)
	wipe32(&k.hpRecv)
}

// deriveEE mixes the ephemeral-ephemeral DH with the static pair binding,
// so a session key can only be derived by holders of both disco privates.
func deriveEE(ephPriv, peerEph, ourPub, peerPub, bind [32]byte) (k sessKeys, err error) {
	shared, err := curve25519.X25519(ephPriv[:], peerEph[:])
	if err != nil {
		return k, err
	}
	defer wipe(shared)
	ikm := make([]byte, 0, len(shared)+len(bind))
	ikm = append(ikm, shared...)
	ikm = append(ikm, bind[:]...)
	defer wipe(ikm)
	k.send = kdf(ikm, ourPub[:], peerPub[:], labelEE)
	k.recv = kdf(ikm, peerPub[:], ourPub[:], labelEE)
	k.tagSend = kdf(ikm, ourPub[:], peerPub[:], labelSessTag)
	k.tagRecv = kdf(ikm, peerPub[:], ourPub[:], labelSessTag)
	k.hpSend = kdf(ikm, ourPub[:], peerPub[:], labelSessHP)
	k.hpRecv = kdf(ikm, peerPub[:], ourPub[:], labelSessHP)
	return k, nil
}

func prf(key [32]byte, parts ...[]byte) [32]byte {
	var out [32]byte
	h, err := blake2s.New256(key[:])
	if err != nil {
		return out
	}
	for _, p := range parts {
		_, _ = h.Write(p)
	}
	copy(out[:], h.Sum(nil))
	return out
}

func hmacEqual(a, b []byte) bool {
	return hmac.Equal(a, b)
}
