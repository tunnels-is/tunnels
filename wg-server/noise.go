package main

// tryDecryptInitiator attempts to recover the initiator's static public key
// from a WireGuard handshake initiation packet using the responder's static
// private key.  It implements the responder-side of Noise_IKpsk2 (WireGuard's
// variant) up to and including decryption of msg.encrypted_static.
//
// Packet layout (148 bytes total):
//
//	[0–3]   message type = 1, 3 zero bytes
//	[4–7]   sender_index (random u32)
//	[8–39]  unencrypted_ephemeral  ← client's ephemeral pubkey
//	[40–87] encrypted_static       ← 32-byte static pubkey + 16-byte AEAD tag
//	[88–115] encrypted_timestamp
//	[116–131] mac1
//	[132–147] mac2
//
// Returns the initiator's 32-byte static public key and true on success.
// Returns false if the packet is malformed or decryption fails (wrong key).

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

// noiseHMAC computes HMAC-BLAKE2s(key, data).
func noiseHMAC(key, data []byte) []byte {
	mac := hmac.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// noiseHash returns BLAKE2s-256(inputs...).
func noiseHash(inputs ...[]byte) []byte {
	h, _ := blake2s.New256(nil)
	for _, b := range inputs {
		h.Write(b)
	}
	return h.Sum(nil)
}

// noiseKDF1 returns the first output of WireGuard's HKDF: T1 = HMAC(HMAC(key, input), 0x1).
func noiseKDF1(key, input []byte) []byte {
	extract := noiseHMAC(key, input)
	return noiseHMAC(extract, []byte{0x1})
}

// noiseKDF2 returns both outputs of WireGuard's HKDF.
func noiseKDF2(key, input []byte) (t1, t2 []byte) {
	extract := noiseHMAC(key, input)
	t1 = noiseHMAC(extract, []byte{0x1})
	t2 = noiseHMAC(extract, append(t1, 0x2))
	return
}

// tryDecryptInitiator recovers the initiator's static public key from pkt.
// serverPriv and serverPub are the responder's Curve25519 keypair (raw 32 bytes).
func tryDecryptInitiator(pkt []byte, serverPriv, serverPub []byte) (pubKeyB64 string, ok bool) {
	if len(pkt) < msgInitiationSize {
		return "", false
	}

	clientEphemeral := pkt[8:40]

	// Step 1–3: derive the initial chaining key and hash state (same for all
	// connections to this server, could be precomputed).
	Ci := noiseHash([]byte(noiseConstruction))
	Hi := noiseHash(noiseHash(Ci, []byte(noiseIdentifier)), serverPub)

	// Step 4–5: mix client's ephemeral into state.
	Ci = noiseKDF1(Ci, clientEphemeral)
	Hi = noiseHash(Hi, clientEphemeral)

	// Step 6: DH(server_static_priv, client_ephemeral_pub).
	shared, err := curve25519.X25519(serverPriv, clientEphemeral)
	if err != nil {
		return "", false
	}

	// Step 7: derive encryption key.
	_, k := noiseKDF2(Ci, shared)

	// Step 8: decrypt encrypted_static using Hi as AAD.
	aead, err := chacha20poly1305.New(k)
	if err != nil {
		return "", false
	}
	nonce := make([]byte, aead.NonceSize()) // all-zero nonce (counter = 0)
	plaintext, err := aead.Open(nil, nonce, pkt[40:88], Hi)
	if err != nil {
		// Decryption failure means this packet wasn't for our server key.
		return "", false
	}

	return base64.StdEncoding.EncodeToString(plaintext), true
}
