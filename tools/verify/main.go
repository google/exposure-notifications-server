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

// This tool verifies that a SHA256 and signature can be valided by the provided public key.
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/asn1"
	"encoding/base64"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
)

var (
	messageDigestFlag = flag.String("digest", "", "base64 encoded sha256 digest of the signed content")
	signatureFlag     = flag.String("signature", "", "signature of the digest param")
	pemFileFlag       = flag.String("pem-file", "", "text file containing the PEM encoded ECDSA public key")
	ieeeFormat        = flag.Bool("ieee1363", false, "if true, signature will be treated as IEEE 1361 instead of ASN1")
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	logger := logging.NewLoggerFromEnv().Named("tools.verify")
	logger = logger.With("build_id", buildinfo.BuildID)
	logger = logger.With("build_tag", buildinfo.BuildTag)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	flag.Parse()
	if *messageDigestFlag == "" {
		return fmt.Errorf("--digest is required and cannot be empty")
	}
	digest, err := base64util.DecodeString(*messageDigestFlag)
	if err != nil {
		return fmt.Errorf("--digest must be a valid base64 encoded sha256 value. error: %w", err)
	}
	if l := len(digest); l != 32 {
		return fmt.Errorf("--digest is not 32 bytes, got: %v", l)
	}
	if *signatureFlag == "" {
		return fmt.Errorf("--signature is required and cannot be empty")
	}
	signature, err := base64util.DecodeString(*signatureFlag)
	if err != nil {
		return fmt.Errorf("--signature must be base64 encoded, error: %w", err)
	}
	pemBytes, err := os.ReadFile(*pemFileFlag)
	if err != nil {
		return fmt.Errorf("--pem-file could not be read: %w", err)
	}

	logger := logging.FromContext(ctx)

	// Validate the signature
	publicKey, err := keys.ParseECDSAPublicKey(string(pemBytes))
	if err != nil {
		return err
	}

	if *ieeeFormat {
		signature, err = convert1363ToAsn1(signature)
		if err != nil {
			return err
		}
		logger.Infow("base64 ASN1 signature", "signature", base64.StdEncoding.EncodeToString(signature))
	}
	if err != nil {
		return fmt.Errorf("unable to convert from IEEE 1363 to ASN1")
	}
	if ecdsa.VerifyASN1(publicKey, digest, signature) {
		logger.Infof("SIGNATURE IS VALID")
	} else {
		logger.Errorf("SIGNATURE IS NOT VALID")
	}

	return nil
}

func convert1363ToAsn1(b []byte) ([]byte, error) {
	rs := struct {
		R, S *big.Int
	}{
		R: new(big.Int).SetBytes(b[:len(b)/2]),
		S: new(big.Int).SetBytes(b[len(b)/2:]),
	}

	return asn1.Marshal(rs)
}
