package main

import (
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/crypto/curve25519"
)

func main() {
	// x, err := crypt.NewEncryptionHandler(3)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(x.SEAL.PrivateKey.PublicKey())
	// fmt.Println(len(x.SEAL.PrivateKey.PublicKey().Bytes()))

	// test()
}

func test() {
	rand.Seed(time.Now().UnixNano())

	var privateKey [32]byte
	for i := range privateKey[:] {
		privateKey[i] = byte(rand.Intn(256))
	}

	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	fmt.Printf("\nAlice Private key (a):\t%x\n", privateKey)
	fmt.Printf("\nAlice Public key point (x co-ord):\t%x\n", publicKey)

	var privateKey2 [32]byte
	for i := range privateKey[:] {
		privateKey2[i] = byte(rand.Intn(256))
	}

	var publicKey2 [32]byte
	curve25519.ScalarBaseMult(&publicKey2, &privateKey2)

	var out1, out2 [32]byte

	fmt.Printf("\nBob Private key (b):\t%x\n", privateKey2)
	fmt.Printf("\nBob Public key point (x co-ord):\t%x\n", publicKey2)

	curve25519.ScalarMult(&out1, &privateKey, &publicKey2)
	curve25519.ScalarMult(&out2, &privateKey2, &publicKey)

	fmt.Printf("\nShared key (Alice):\t%x\n", out1)
	fmt.Printf("\nShared key (Bob):\t%x\n", out2)
}
