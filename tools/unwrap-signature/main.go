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

// This utility unwrapps the export.TEKSignatureList proto and
// extracts the sianature so that an export file can be verified with openssl.
package main

import (
	"flag"
	"io/ioutil"
	"log"

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

	inData, err := ioutil.ReadFile(*inFile)
	if err != nil {
		log.Fatalf("unable to read input file: %v", err)
	}

	teksl := &export.TEKSignatureList{}
	proto.Unmarshal(inData, teksl)

	log.Printf("Data: \n%v", teksl)
	sig := teksl.Signatures[0].Signature

	err = ioutil.WriteFile(*outFile, sig, 0600)
	if err != nil {
		log.Fatalf("unable to write output file: %v", err)
	}
	log.Printf("success, saved output to %v", *outFile)
}
