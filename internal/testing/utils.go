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

package testing

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	vm "github.com/google/exposure-notifications-server/internal/verification/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	utils "github.com/google/exposure-notifications-server/pkg/verification"
)

// JwtConfig stores the config used to fetch a verification jwt certificate
type JwtConfig struct {
	HealthAuthority      *vm.HealthAuthority
	HealthAuthorityKey   *vm.HealthAuthorityKey
	ExposureKeys         []verifyapi.ExposureKey
	Key                  *ecdsa.PrivateKey
	JWTWarp              time.Duration
	ReportType           string
	SymptomOnsetInterval uint32
}

// IssueJWT generates a JWT as if it came from the
// authorized health authority.
func IssueJWT(t *testing.T, cfg JwtConfig) (jwtText, hmacKey string) {
	t.Helper()

	hmacKeyBytes := make([]byte, 32)
	if _, err := rand.Read(hmacKeyBytes); err != nil {
		t.Fatal(err)
	}
	hmacKey = base64.StdEncoding.EncodeToString(hmacKeyBytes)

	hmacBytes, err := utils.CalculateExposureKeyHMAC(cfg.ExposureKeys, hmacKeyBytes)
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
