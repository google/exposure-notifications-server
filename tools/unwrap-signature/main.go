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

// This utility unwraps the export.TEKSignatureList proto and
// extracts the signature so that an export file can be verified with openssl.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/google/exposure-notifications-server/internal/pb/export"
	"google.golang.org/protobuf/proto"
)

var (
	inFile  = flag.String("in", "", "input file, containing sig proto")
	outFile = flag.String("out", "", "output file, just signature")
)

func main() {
	flag.Parse()

	if *inFile == "" {
		log.Fatalf("no in file passed in, --in")
	}
	if *outFile == "" {
		log.Fatalf("no out file passed in, --out")
	}

	inData, err := os.ReadFile(*inFile)
	if err != nil {
		log.Fatalf("unable to read input file: %v", err)
	}

	teksl := &export.TEKSignatureList{}
	if err := proto.Unmarshal(inData, teksl); err != nil {
		log.Fatalf("failed to unmarshal proto: %v", err)
	}

	log.Printf("Data: \n%v", teksl)
	sig := teksl.Signatures[0].Signature

	err = os.WriteFile(*outFile, sig, 0o600)
	if err != nil {
		log.Fatalf("unable to write output file: %v", err)
	}
	log.Printf("success, saved output to %v", *outFile)
}
