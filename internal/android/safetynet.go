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

// package android managed device attestation inegation with Android's
// SafetyNet API.
package android

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"runtime/trace"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"

	"github.com/dgrijalva/jwt-go"
)

// The VerifyOpts determine the fields that are required for verification
type VerifyOpts struct {
	AppPkgName      string
	APKDigest       string
	Nonce           Noncer
	CTSProfileMatch bool
	BasicIntegrity  bool
	MinValidTime    *time.Time
	MaxValidTime    *time.Time
}

//, appPackageName string, base64keys []string, regions []string

func ValidateAttestation(ctx context.Context, attestation string, opts VerifyOpts) error {
	defer trace.StartRegion(ctx, "ValidateAttestation").End()
	logger := logging.FromContext(ctx)

	claims, err := verifyAttestation(ctx, attestation)
	if err != nil {
		return fmt.Errorf("verifyAttestation: %w", err)
	}

	// Validate the nonce.
	if opts.Nonce == nil || opts.Nonce.Nonce() == "" {
		return fmt.Errorf("missing nonce")
	}
	nonceClaimB64, ok := claims["nonce"].(string)
	if !ok {
		return fmt.Errorf("invalid nonce claim, not a string")
	}
	nonceClaimBytes, err := base64.StdEncoding.DecodeString(nonceClaimB64)
	if err != nil {
		return fmt.Errorf("unable to decode nonce claim data: %w", err)
	}
	nonceClaim := string(nonceClaimBytes)
	nonceCalculated := opts.Nonce.Nonce()
	if nonceCalculated != nonceClaim {
		return fmt.Errorf("nonce mismatch: expected %v got %v", nonceCalculated, nonceClaim)
	}

	// Validate time interval.
	if opts.MinValidTime == nil || opts.MinValidTime.IsZero() ||
		opts.MaxValidTime == nil || opts.MaxValidTime.IsZero() {
		return fmt.Errorf("missing timestamp bounds for attestation")
	}
	issMsF, ok := claims["timestampMs"].(float64)
	if !ok {
		return fmt.Errorf("timestampMs is not a readable value: %v", claims["timestampMs"])
	}
	issueTime := time.Unix(int64(issMsF/1000), 0)

	if opts.MinValidTime != nil && opts.MinValidTime.Unix() > issueTime.Unix() {
		return fmt.Errorf("attestation is too old, must be newer than %v, was %v", opts.MinValidTime.Unix(), issueTime.Unix())
	}
	if opts.MaxValidTime != nil && opts.MaxValidTime.Unix() < issueTime.Unix() {
		return fmt.Errorf("attestation is in the future, must be older than %v, was %v", opts.MaxValidTime.Unix(), issueTime.Unix())
	}

	// TODO(mikehelmick): Validate APKDigest
	logger.Warnf("attestation, apkCertificateDigestSha256 validation not implemented")

	// Integrity checks.
	if opts.CTSProfileMatch {
		ctsProfileMatch, ok := claims["ctsProfileMatch"].(bool)
		if !ok {
			return fmt.Errorf("attestation value of ctsProfileMatch is not a valid bool, %v", claims["ctsProfileMatch"])
		}
		if !ctsProfileMatch {
			return fmt.Errorf("ctsProfileMatch is false when true is required")
		}
	} else {
		logger.Warnf("Verify attestation is not checking ctsProfileMatch")
	}

	if opts.BasicIntegrity {
		basicIntegrity, ok := claims["basicIntegrity"].(bool)
		if !ok {
			return fmt.Errorf("attestation value of basicIntegrity is not a valid bool, %v", claims["basicIntegrity"])
		}
		if !basicIntegrity {
			return fmt.Errorf("basicIntegrity is false when true is required")
		}
	}

	return nil
}

// The keyFunc is based on the Android sample code
// https://github.com/googlesamples/android-play-safetynet/blob/d7513a54e2f28c0dcd7f8d8d0fa03adb5d87b91a/server/java/src/main/java/OfflineVerify.java
func keyFunc(ctx context.Context, tok *jwt.Token) (interface{}, error) {
	x5c, ok := tok.Header["x5c"].([]interface{})
	if !ok || len(x5c) == 0 {
		return nil, fmt.Errorf("attestation is missing certificate")
	}

	// Verify the sigature of the JWS and retrieve the signature and certificates.
	x509certs := make([]*x509.Certificate, len(x5c))
	for i, certStr := range x5c {
		if certStr == "" {
			return nil, fmt.Errorf("certificate is empty")
		}
		certData, err := base64.StdEncoding.DecodeString(certStr.(string))
		if err != nil {
			return nil, fmt.Errorf("invalid certificate encoding: %w", err)
		}
		x509certs[i], err = x509.ParseCertificate(certData)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate: %w", err)
		}
	}

	pool := x509.NewCertPool()
	for _, cert := range x509certs {
		pool.AddCert(cert)
	}
	opts := x509.VerifyOptions{
		DNSName:       "attest.android.com", // required hostname for valid attestation.
		Intermediates: pool,
	}

	// Verify the first certificate, with all added as allowed intermediates.
	_, err := x509certs[0].Verify(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate chain: %w", err)
	}

	// extract the public key for verification.
	if rsaKey, ok := x509certs[0].PublicKey.(*rsa.PublicKey); ok {
		return rsaKey, nil
	}
	return nil, fmt.Errorf("invalid certificate, unable to extract public key")
}

// verifyAttestation extracts and verifies the signature and claims on the
// attestation. It does NOT validate the attestation, only the signature.
func verifyAttestation(ctx context.Context, signedAttestation string) (jwt.MapClaims, error) {
	defer trace.StartRegion(ctx, "verifyAttestation").End()
	// jwt.Parse also validates the signature after extracting
	// the key via the keyFunc, which validates the certificate chain.
	token, err := jwt.Parse(signedAttestation,
		func(tok *jwt.Token) (interface{}, error) {
			return keyFunc(ctx, tok)
		})

	if err != nil {
		return nil, fmt.Errorf("jwt.Parse: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid JWS attestation")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("claims are of wrong type")
	}
	return claims, nil
}
