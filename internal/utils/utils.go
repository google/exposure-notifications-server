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

// Package verification provides verification utilities.
//
// This is provided as reference to application authors wishing to calculate
// the exposure key HMAC as part of their exposure notifications mobile app.
//
// This protocol is detailed at
// https://developers.google.com/android/exposure-notifications/verification-system
//

// Package utils provides utilities to be used in testing.
package utils

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/exposure-notifications-server/internal/database"
	vdb "github.com/google/exposure-notifications-server/internal/verification/database"
	vm "github.com/google/exposure-notifications-server/internal/verification/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	vutil "github.com/google/exposure-notifications-server/pkg/verification"
)

// SigningKey holds a single signing key and the PEM public key.
type SigningKey struct {
	Key       *ecdsa.PrivateKey
	PublicKey string
}

// JWTConfig stores the config used to fetch a verification jwt certificate
type JWTConfig struct {
	HealthAuthority      *vm.HealthAuthority
	HealthAuthorityKey   *vm.HealthAuthorityKey
	ExposureKeys         []verifyapi.ExposureKey
	Key                  *ecdsa.PrivateKey
	JWTWarp              time.Duration
	ReportType           string
	SymptomOnsetInterval uint32
}

// GetSigningKey returns a new signing key to be used for verification.
func GetSigningKey(tb testing.TB) *SigningKey {
	tb.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}

	publicKey := privateKey.Public()
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		tb.Fatal(err)
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})
	pemPublicKey := string(pemEncodedPub)

	return &SigningKey{
		Key:       privateKey,
		PublicKey: pemPublicKey,
	}
}

// IssueJWT generates a JWT as if it came from the
// authorized health authority.
func IssueJWT(t *testing.T, cfg JWTConfig) (jwtText, hmacKey string) {
	t.Helper()

	hmacKeyBytes := make([]byte, 32)
	if _, err := rand.Read(hmacKeyBytes); err != nil {
		t.Fatal(err)
	}
	hmacKey = base64.StdEncoding.EncodeToString(hmacKeyBytes)

	hmacBytes, err := vutil.CalculateExposureKeyHMAC(cfg.ExposureKeys, hmacKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	hmac := base64.StdEncoding.EncodeToString(hmacBytes)

	claims := verifyapi.NewVerificationClaims()
	claims.Audience = cfg.HealthAuthority.Audience
	claims.Issuer = cfg.HealthAuthority.Issuer
	claims.IssuedAt = time.Now().Add(cfg.JWTWarp).Unix()
	claims.ExpiresAt = time.Now().Add(cfg.JWTWarp).Add(5 * time.Minute).Unix()
	claims.SignedMAC = hmac
	if cfg.ReportType != "" {
		claims.ReportType = cfg.ReportType
	}
	if cfg.SymptomOnsetInterval > 0 {
		claims.SymptomOnsetInterval = cfg.SymptomOnsetInterval
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header[verifyapi.KeyIDHeader] = cfg.HealthAuthorityKey.Version
	jwtText, err = token.SignedString(cfg.Key)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// BuildJWTConfig creates a stub for a generic JWT given an existing database and
// exposure keys
func BuildJWTConfig(tb testing.TB, db *database.DB, keys []verifyapi.ExposureKey) *JWTConfig {
	ctx := context.Background()
	verifyDB := vdb.New(db)

	// create a signing key
	sk := GetSigningKey(tb)

	// create a health authority
	ha := &vm.HealthAuthority{
		Audience: "exposure-notifications-service",
		Issuer:   "Department of Health",
		Name:     "Integration Test HA",
	}
	haKey := &vm.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}
	haKey.PublicKeyPEM = sk.PublicKey
	verifyDB.AddHealthAuthority(ctx, ha)
	verifyDB.AddHealthAuthorityKey(ctx, ha, haKey)

	// jwt config to be used to get a verification certificate
	return &JWTConfig{
		HealthAuthority:    ha,
		HealthAuthorityKey: haKey,
		ExposureKeys:       keys,
		Key:                sk.Key,
		JWTWarp:            time.Duration(0),
		ReportType:         verifyapi.ReportTypeConfirmed,
	}
}
