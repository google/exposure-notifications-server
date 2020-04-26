package main

import (
	"encoding/base64"
	"encoding/hex"
	"log"
	"os"
)

// Small utility that converts the first argument from a hex encoded
// byte array into the 16 byte temporary trace key
// and then base 64 encodes it.
//
// Useful for converting test data.
func main() {
	if len(os.Args) != 2 {
		log.Fatal("requires 1 argument which will be interepreted as a hex byte array")
	}

	hexKey := os.Args[1]
	bytes, err := hex.DecodeString(hexKey)
	if len(bytes) != 16 {
		log.Fatalf("decoded hex string, want len(bytes)=16, got: %v", len(bytes))
	}
	if err != nil {
		log.Fatalf("hex.DecodeString: %v : %v", hexKey, err)
	}
	// The client application doesn't pad the keys.
	keyBase64 := base64.RawStdEncoding.EncodeToString(bytes)

	log.Printf("ttk: %v", bytes)
	log.Printf("base64(ttk): %v", keyBase64)
}
