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
// Although exported, this package is non intended for general consumption.
// It is a shared dependency between multiple exposure notifications projects.
// We cannot guarantee that there won't be breaking changes in the future.
package verification

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	vm "github.com/google/exposure-notifications-server/internal/verification/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
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

// CalculateExpsureKeyHMACv1Alpha1 is a convenience method for anyone still on v1alpha1.
// Deprecated: use CalculateExposureKeyHMAC instead
// Preserved for clients on v1alpha1, will be removed in v0.3 release.
func CalculateExpsureKeyHMACv1Alpha1(legacyKeys []v1alpha1.ExposureKey, secret []byte) ([]byte, error) {
	keys := make([]verifyapi.ExposureKey, len(legacyKeys))
	for i, k := range legacyKeys {
		keys[i] = verifyapi.ExposureKey{
			Key:              k.Key,
			IntervalNumber:   k.IntervalNumber,
			IntervalCount:    k.IntervalCount,
			TransmissionRisk: k.TransmissionRisk,
		}
	}
	return CalculateExposureKeyHMAC(keys, secret)
}

// CalculateExposureKeyHMAC will calculate the verification protocol HMAC value.
// Input keys are already to be base64 encoded. They will be sorted if necessary.
func CalculateExposureKeyHMAC(keys []verifyapi.ExposureKey, secret []byte) ([]byte, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot calculate hmac on empty exposure keys")
	}
	// Sort by the key
	sort.Slice(keys, func(i int, j int) bool {
		return strings.Compare(keys[i].Key, keys[j].Key) <= 0
	})

	// Build the cleartext.
	perKeyText := make([]string, 0, len(keys))
	for _, ek := range keys {
		perKeyText = append(perKeyText,
			fmt.Sprintf("%s.%d.%d.%d", ek.Key, ek.IntervalNumber, ek.IntervalCount, ek.TransmissionRisk))
	}

	cleartext := strings.Join(perKeyText, ",")
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(cleartext)); err != nil {
		return nil, fmt.Errorf("failed to write hmac: %w", err)
	}

	return mac.Sum(nil), nil
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

	hmacBytes, err := CalculateExposureKeyHMAC(cfg.ExposureKeys, hmacKeyBytes)
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
