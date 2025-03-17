package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tunnels-is/tunnels/coretwo/pkg/crypto"
)

func main() {
	// Define command-line flags
	generateKey := flag.Bool("generate", false, "Generate a new secure key")
	keyLengthBytes := flag.Int("length", 32, "Length of the key in bytes (16, 24, or 32)")
	format := flag.String("format", "hex", "Output format (hex, base64, or raw)")
	testKey := flag.String("test", "", "Test key by encrypting and decrypting a message")
	testMessage := flag.String("message", "Hello, World!", "Message to test with encryption")
	flag.Parse()

	// Generate a new key
	if *generateKey {
		key := make([]byte, *keyLengthBytes)
		_, err := rand.Read(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
			os.Exit(1)
		}

		outputKey(key, *format)
		return
	}

	// Test an existing key
	if *testKey != "" {
		// Convert key from specified format
		var keyBytes []byte
		var err error

		switch {
		case strings.HasPrefix(*testKey, "hex:"):
			hexKey := (*testKey)[4:] // Remove "hex:" prefix
			keyBytes, err = hex.DecodeString(hexKey)
		case strings.HasPrefix(*testKey, "base64:"):
			b64Key := (*testKey)[7:] // Remove "base64:" prefix
			keyBytes, err = base64.StdEncoding.DecodeString(b64Key)
		default:
			// Treat as a string key
			keyBytes = []byte(*testKey)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding key: %v\n", err)
			os.Exit(1)
		}

		// Create crypto key
		k, err := crypto.NewKey(string(keyBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating key: %v\n", err)
			os.Exit(1)
		}

		// Encrypt the test message
		plaintext := []byte(*testMessage)
		ciphertext, err := k.Encrypt(plaintext)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encrypting message: %v\n", err)
			os.Exit(1)
		}

		// Decrypt the message
		decrypted, err := k.Decrypt(ciphertext)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decrypting message: %v\n", err)
			os.Exit(1)
		}

		// Verify the message
		if string(decrypted) != *testMessage {
			fmt.Fprintf(os.Stderr, "Decryption failed: expected '%s', got '%s'\n", *testMessage, decrypted)
			os.Exit(1)
		}

		// Print results
		fmt.Println("Encryption test successful!")
		fmt.Printf("Original: %s\n", *testMessage)
		fmt.Printf("Encrypted (hex): %x\n", ciphertext)
		fmt.Printf("Decrypted: %s\n", decrypted)

		return
	}

	// If no actions were specified, print usage
	flag.Usage()
}

func outputKey(key []byte, format string) {
	switch format {
	case "hex":
		fmt.Printf("hex:%s\n", hex.EncodeToString(key))
	case "base64":
		fmt.Printf("base64:%s\n", base64.StdEncoding.EncodeToString(key))
	case "raw":
		fmt.Printf("%s\n", key)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", format)
		os.Exit(1)
	}
}
