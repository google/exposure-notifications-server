// Copyright 2020 Google LLC
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
	"encoding/hex"
	"log"
	"os"

	"github.com/google/exposure-notifications-server/pkg/base64util"
)

// Small to base64 decode data and print some info
func main() {
	if len(os.Args) != 2 {
		log.Fatal("requires 1 argument which will be interpreted as a base64 encoded byte array")
	}

	encoded := os.Args[1]
	bytes, err := base64util.DecodeString(encoded)
	if err != nil {
		log.Fatalf("base64util.DecodeString: %v : %v", encoded, err)
	}
	log.Printf("Number of bytes: %v", len(bytes))

	hex := hex.EncodeToString(bytes)
	log.Printf("As HEX: %v", hex)
}
