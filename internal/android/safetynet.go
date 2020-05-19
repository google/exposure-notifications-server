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

// Package android managed device attestation inegation with Android's
// SafetyNet API.
package android

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"runtime/trace"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"

	"github.com/dgrijalva/jwt-go"
)

// The VerifyOpts determine the fields that are required for verification
type VerifyOpts struct {
	AppPkgName      string
	APKDigest       []string
	Nonce           string
	CTSProfileMatch bool
	BasicIntegrity  bool
	MinValidTime    time.Time
	MaxValidTime    time.Time
}

// ValidateAttestation validates the the SafetyNet Attestation from this device
// matches the properties that we expect based on the AuthorizedApp entry. See
// https://developer.android.com/training/safetynet/attestation#use-response-server
// for details on the format of these attestations.
func ValidateAttestation(ctx context.Context, attestation string, opts *VerifyOpts) error {
	defer trace.StartRegion(ctx, "ValidateAttestation").End()
	logger := logging.FromContext(ctx)

	claims, err := verifyAttestation(ctx, attestation)
	if err != nil {
		return fmt.Errorf("verifyAttestation: %w", err)
	}

	// Validate the nonce.
	if opts.Nonce == "" {
		return fmt.Errorf("missing nonce")
	}
	nonceClaimB64, ok := claims["nonce"].(string)
	if !ok {
		return fmt.Errorf("invalid nonce claim, not a string")
	}
	nonceClaimBytes, err := base64util.DecodeString(nonceClaimB64)
	if err != nil {
		return fmt.Errorf("unable to decode nonce claim data: %w", err)
	}
	nonceClaim := string(nonceClaimBytes)
	nonceCalculated := opts.Nonce
	if nonceCalculated != nonceClaim {
		return fmt.Errorf("nonce mismatch: expected %v got %v", nonceCalculated, nonceClaim)
	}

	// Validate time interval.
	if opts.MinValidTime.IsZero() || opts.MaxValidTime.IsZero() {
		return fmt.Errorf("missing timestamp bounds for attestation")
	}
	issMsF, ok := claims["timestampMs"].(float64)
	if !ok {
		return fmt.Errorf("timestampMs is not a readable value: %v", claims["timestampMs"])
	}
	issueTime := time.Unix(int64(issMsF/1000), 0)

	if opts.MinValidTime.Unix() > issueTime.Unix() {
		return fmt.Errorf("attestation is too old, must be newer than %v, was %v", opts.MinValidTime.Unix(), issueTime.Unix())
	}
	if opts.MaxValidTime.Unix() < issueTime.Unix() {
		return fmt.Errorf("attestation is in the future, must be older than %v, was %v", opts.MaxValidTime.Unix(), issueTime.Unix())
	}

	// The apkCertificateDigestSha256 is an array with a single entry.
	// https://developer.android.com/training/safetynet/attestation#use-response-server
	digestArr := claims["apkCertificateDigestSha256"].([]interface{})
	claimApkDigest := ""
	if len(digestArr) >= 1 {
		claimApkDigest = digestArr[0].(string)
	} else {
		logger.Warnf("attestation didn't contain apkCertificateDigestSha256")
	}

	match := false
	for _, digest := range opts.APKDigest {
		if digest != claimApkDigest {
			match = true
			break
		}
	}
	if !match {
		return fmt.Errorf("attestation apkCertificateDigestSha256 value does not match configuration, got %v", claimApkDigest)
	}

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

// VerifyOptsFor returns the Android SafetyNet verification options to be used
// based on the AuthorizedApp configuration, request time, and nonce.
func VerifyOptsFor(c *database.AuthorizedApp, from time.Time, nonce string) *VerifyOpts {
	digests := make([]string, len(c.ApkDigestSHA256))
	copy(digests, c.ApkDigestSHA256)
	rtn := &VerifyOpts{
		AppPkgName:      c.AppPackageName,
		CTSProfileMatch: c.CTSProfileMatch,
		BasicIntegrity:  c.BasicIntegrity,
		APKDigest:       digests,
		Nonce:           nonce,
	}

	// Calculate the valid time window based on now + config options.
	if c.AllowedPastTime > 0 {
		minTime := from.Add(-c.AllowedPastTime)
		rtn.MinValidTime = minTime
	}
	if c.AllowedFutureTime > 0 {
		maxTime := from.Add(c.AllowedFutureTime)
		rtn.MaxValidTime = maxTime
	}

	return rtn
}

// The keyFunc is based on the Android sample code
// https://github.com/googlesamples/android-play-safetynet/blob/d7513a54e2f28c0dcd7f8d8d0fa03adb5d87b91a/server/java/src/main/java/OfflineVerify.java
func keyFunc(ctx context.Context, tok *jwt.Token) (interface{}, error) {
	x5c, ok := tok.Header["x5c"].([]interface{})
	if !ok || len(x5c) == 0 {
		return nil, fmt.Errorf("attestation is missing certificate")
	}

	// Verify the singature of the JWS and retrieve the signature and certificates.
	x509certs := make([]*x509.Certificate, len(x5c))
	for i, certStr := range x5c {
		if certStr == "" {
			return nil, fmt.Errorf("certificate is empty")
		}
		certData, err := base64util.DecodeString(certStr.(string))
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
