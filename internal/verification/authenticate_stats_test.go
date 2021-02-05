// Copyright 2021 Google LLC
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

package verification

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"

	"github.com/dgrijalva/jwt-go"
)

type ClaimChanger func(*jwt.StandardClaims) *jwt.StandardClaims

func claimsIdentity(claims *jwt.StandardClaims) *jwt.StandardClaims {
	return claims
}

type HeaderChanger func(map[string]interface{})

func headerIdentity(headers map[string]interface{}) {
}

type JWTChanger func(string) string

func jwtIdentity(jwt string) string {
	return jwt
}

func TestAuthenticateStatsToken(t *testing.T) {
	t.Parallel()

	statsAudience := "test-stats-aud"

	cases := []struct {
		Name         string
		Warp         time.Duration
		ModifyClaims ClaimChanger
		ModifyHeader HeaderChanger
		ModifyJWT    JWTChanger
		DisableAPI   bool
		Error        string
	}{
		{
			Name:         "valid_token",
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
		},
		{
			Name:         "missing_kid",
			ModifyClaims: claimsIdentity,
			ModifyHeader: func(header map[string]interface{}) {
				delete(header, "kid")
			},
			ModifyJWT: jwtIdentity,
			Error:     "missing 'kid' header in token",
		},
		{
			Name: "wrong_issuer",
			ModifyClaims: func(claims *jwt.StandardClaims) *jwt.StandardClaims {
				claims.Issuer = "nope"
				return claims
			},
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
			Error:        "issuer not found: nope",
		},
		{
			Name:         "wrong_kid",
			ModifyClaims: claimsIdentity,
			ModifyHeader: func(header map[string]interface{}) {
				header["kid"] = "v99"
			},
			ModifyJWT: jwtIdentity,
			Error:     "key not found: kid: v99",
		},
		{
			Name:         "not_valid_yet",
			Warp:         time.Hour,
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
			Error:        "token is not valid yet",
		},
		{
			Name:         "expired",
			Warp:         -1 * time.Hour,
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
			Error:        "token is expired by ",
		},
		{
			Name: "wrong_audience",
			ModifyClaims: func(claims *jwt.StandardClaims) *jwt.StandardClaims {
				claims.Audience = "nope"
				return claims
			},
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
			Error:        "unauthorized, audience mismatch",
		},
		{
			Name:         "empty_jwt",
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT: func(jwt string) string {
				return ""
			},
			Error: "token contains an invalid number of segments",
		},
		{
			Name:         "too_many_segments",
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT: func(jwt string) string {
				return fmt.Sprintf("%s.%s", jwt, "extra")
			},
			Error: "token contains an invalid number of segments",
		},
		{
			Name:         "forbidden",
			ModifyClaims: claimsIdentity,
			ModifyHeader: headerIdentity,
			ModifyJWT:    jwtIdentity,
			DisableAPI:   true,
			Error:        "API access forbidden",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := project.TestContext(t)
			// Set up database. Create HealthAuthority + HAKey for the test.
			testDB, _ := testDatabaseInstance.NewDatabase(t)
			haDB := database.New(testDB)

			// For each test case - we need to create a health authority and a valid key.
			// Generate ECDSA key pair.
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			publicKey := privateKey.Public()

			// Get the PEM for the public key.
			x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
			if err != nil {
				t.Fatal(err)
			}
			pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})
			pemPublicKey := string(pemEncodedPub)

			issuer := fmt.Sprintf("issuer-%s", t.Name())
			audience := fmt.Sprintf("aud-%s", t.Name())
			healthAuthority := model.HealthAuthority{
				Issuer:         issuer,
				Audience:       audience,
				Name:           "Very Real Health Authority",
				EnableStatsAPI: true,
			}
			if tc.DisableAPI {
				healthAuthority.EnableStatsAPI = false
			}
			if err := haDB.AddHealthAuthority(ctx, &healthAuthority); err != nil {
				t.Fatal(err)
			}

			hak := model.HealthAuthorityKey{
				Version:      "v1",
				From:         time.Now(),
				PublicKeyPEM: pemPublicKey,
			}
			if err := haDB.AddHealthAuthorityKey(ctx, &healthAuthority, &hak); err != nil {
				t.Fatal(err)
			}

			now := time.Now().UTC().Add(tc.Warp)
			claims := &jwt.StandardClaims{
				Audience:  statsAudience,
				ExpiresAt: now.Add(time.Minute).Unix(),
				IssuedAt:  now.Unix(),
				Issuer:    issuer,
				NotBefore: now.Unix(),
			}
			claims = tc.ModifyClaims(claims)

			token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
			token.Header["kid"] = "v1"
			tc.ModifyHeader(token.Header)

			jwtString, err := token.SignedString(privateKey)
			if err != nil {
				t.Fatalf("unable to sign JWT: %v", err)
			}
			jwtString = tc.ModifyJWT(jwtString)

			verifier, err := New(haDB, &Config{time.Nanosecond, statsAudience})
			if err != nil {
				t.Fatal(err)
			}

			gotID, err := verifier.AuthenticateStatsToken(ctx, jwtString)

			if err != nil {
				if tc.Error == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("wanted error '%v', got error '%v'", tc.Error, err.Error())
				}
			} else if tc.Error != "" {
				t.Fatalf("wanted error '%v', but got nil", tc.Error)
			}

			if tc.Error == "" {
				if gotID != healthAuthority.ID {
					t.Fatalf("incorrect health authority id want: %v, got: %v", healthAuthority.ID, gotID)
				}
			}
		})
	}
}
