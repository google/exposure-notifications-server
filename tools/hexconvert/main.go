// Copyright 2020 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		log.Fatal("requires 1 argument which will be interpreted as a hex byte array")
	}

	hexKey := os.Args[1]
	bytes, err := hex.DecodeString(hexKey)
	if err != nil {
		log.Fatalf("hex.DecodeString: %v : %v", hexKey, err)
	}
	if len(bytes) != 16 {
		log.Fatalf("decoded hex string, want len(bytes)=16, got: %v", len(bytes))
	}
	// The client application doesn't pad the keys.
	keyBase64 := base64.RawStdEncoding.EncodeToString(bytes)

	log.Printf("ttk: %v", bytes)
	log.Printf("base64(ttk): %v", keyBase64)
}
