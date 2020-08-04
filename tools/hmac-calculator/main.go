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

// Unmarshals a public JSON message from a file and calculated the HMAC using
// the server code. Does NOT validate certificate signature.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/verification"
)

const (
	bufferSize = 32000
)

func main() {
	fileFlag := flag.String("file", "", "--file=<filename that contains publish json>")
	flag.Parse()

	if *fileFlag == "" {
		log.Fatalf("missing --file arg, don't know what to marshal.")
	}

	publish, err := ReadFile(*fileFlag)
	if err != nil {
		log.Fatalf("Error parsing request from file: %v", err)
	}

	secret, err := base64util.DecodeString(publish.HMACKey)
	if err != nil {
		log.Fatalf("unable to decode hmac secret: %v", err)
	}
	wantHMAC, err := verification.CalculateExposureKeyHMAC(publish.Keys, secret)
	if err != nil {
		log.Fatalf("error calculating hmac: %v", err)
	}

	log.Printf("Expected HMAC (raw): %v", wantHMAC)
	log.Printf("Expected HMAC B64: %v", base64.StdEncoding.EncodeToString(wantHMAC))
}

func ReadFile(fname string) (*verifyapi.Publish, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 32000)
	n, err := f.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v, %w", fname, err)
	}
	if n == bufferSize {
		return nil, fmt.Errorf("file too large: %v - more than %v bytes", fname, bufferSize)
	}

	var publish verifyapi.Publish
	if err := json.Unmarshal(buffer[0:n], &publish); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &publish, nil
}
