package wgserver

import (
	"crypto/hmac"
	"crypto/rand"
	"errors"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	cookieRefresh          = 2 * time.Minute
	msgCookieReplyType     = 3
	msgCookieReplySize     = 64
	maxCookieRepliesPerSec = 1000
)

var errCookieBudget = errors.New("cookie reply budget")

func (b *LazyBind) validMAC2(pkt, src []byte) bool {
	if len(pkt) < msgInitiationSize || len(src) == 0 {
		return false
	}
	cookie, ok := b.cookieFor(src)
	if !ok {
		return false
	}
	mac, err := blake2s.New128(cookie[:])
	if err != nil {
		return false
	}
	mac.Write(pkt[:msgInitiationSize-blake2s.Size128])
	var sum [blake2s.Size128]byte
	mac.Sum(sum[:0])
	return hmac.Equal(sum[:], pkt[msgInitiationSize-blake2s.Size128:])
}

func (b *LazyBind) cookieFor(src []byte) ([blake2s.Size128]byte, bool) {
	var cookie [blake2s.Size128]byte
	b.cookieMu.Lock()
	defer b.cookieMu.Unlock()
	if b.cookieSecretAt.IsZero() || time.Since(b.cookieSecretAt) > cookieRefresh {
		return cookie, false
	}
	mac, err := blake2s.New128(b.cookieSecret[:])
	if err != nil {
		return cookie, false
	}
	mac.Write(src)
	mac.Sum(cookie[:0])
	return cookie, true
}

func (b *LazyBind) writeCookieReply(pkt, src []byte) ([]byte, error) {
	if len(pkt) < msgInitiationSize || len(src) == 0 {
		return nil, errors.New("short handshake")
	}
	var secret [blake2s.Size]byte
	b.cookieMu.Lock()
	now := time.Now()
	sec := now.Unix()
	if b.cookieSec != sec {
		b.cookieSec = sec
		b.cookieSent = 0
	}
	if b.cookieSent >= maxCookieRepliesPerSec {
		b.cookieMu.Unlock()
		return nil, errCookieBudget
	}
	b.cookieSent++
	if b.cookieSecretAt.IsZero() || now.Sub(b.cookieSecretAt) > cookieRefresh {
		if _, err := rand.Read(b.cookieSecret[:]); err != nil {
			b.cookieMu.Unlock()
			return nil, err
		}
		b.cookieSecretAt = now
	}
	secret = b.cookieSecret
	b.cookieMu.Unlock()

	var cookie [blake2s.Size128]byte
	mac, err := blake2s.New128(secret[:])
	if err != nil {
		return nil, err
	}
	mac.Write(src)
	mac.Sum(cookie[:0])

	key := noiseHash([]byte(labelCookie), b.serverPub)
	defer zeroBytes(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, cookie[:], pkt[msgInitiationSize-2*blake2s.Size128:msgInitiationSize-blake2s.Size128])
	if len(sealed) != blake2s.Size128+16 {
		return nil, errors.New("cookie seal length")
	}

	reply := make([]byte, msgCookieReplySize)
	reply[0] = msgCookieReplyType
	copy(reply[4:8], pkt[4:8])
	copy(reply[8:8+chacha20poly1305.NonceSizeX], nonce)
	copy(reply[8+chacha20poly1305.NonceSizeX:], sealed)
	return reply, nil
}
