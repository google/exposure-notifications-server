// Copyright 2021 the Exposure Notifications Server authors
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

// This tool verifies that a SHA256 and signature can be valided by the provided public key.
package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

var (
	filePath    = flag.String("file", "", "path to the export files, supports file globs")
	keyID       = flag.String("key-id", "", "the expected key ID")
	keyVersion  = flag.String("key-version", "", "the expected key version")
	pemFileFlag = flag.String("pem-file", "", "text file containing the PEM encoded ECDSA public key")
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	err := realMain(ctx)
	done()

	if err != nil {
		log.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	// Check flags.
	flag.Parse()

	if *keyID == "" {
		return fmt.Errorf("--key-id must be provided")
	}
	if *keyVersion == "" {
		return fmt.Errorf("--key-version must be provided")
	}

	pemBytes, err := os.ReadFile(*pemFileFlag)
	if err != nil {
		return fmt.Errorf("--pem-file could not be read: %w", err)
	}

	// Parse the public key.
	publicKey, err := keys.ParseECDSAPublicKey(string(pemBytes))
	if err != nil {
		return err
	}

	// Read export file.
	blob, err := os.ReadFile(*filePath)
	if err != nil {
		return fmt.Errorf("can't read export file: %q: %w", *filePath, err)
	}

	// Load valid signatures form export.sig.
	signatureBlock, err := export.UnmarshalSignatureFile(blob)
	if err != nil {
		return fmt.Errorf("unable to read signature block: %w", err)
	}

	// Get the digest of export.bin (contents don't matter)
	_, digest, err := export.UnmarshalExportFile(blob)
	if err != nil {
		return fmt.Errorf("unable to read export block: %w", err)
	}

	var signature []byte
	// Pull a matching signature if found.
	for _, tekSig := range signatureBlock.GetSignatures() {
		if tekSig.SignatureInfo.GetVerificationKeyId() == *keyID && tekSig.SignatureInfo.GetVerificationKeyVersion() == *keyVersion {
			signature = tekSig.GetSignature()
		}
	}
	if len(signature) == 0 {
		return fmt.Errorf("unable to find signature matching keyID: %q and keyVersion %q", *keyID, *keyVersion)
	}

	// Validate signatures
	if ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		log.Printf("valid signature, file: %q keyID: %q keyVersion: %q", *filePath, *keyID, *keyVersion)
		return nil
	}

	return fmt.Errorf("signature did not verify")
}
