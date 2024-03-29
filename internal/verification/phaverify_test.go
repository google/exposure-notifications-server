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

// Package verification provides the ability to verify the diagnosis certificates
// (JWTs) coming from public health authorities that are responsible for verifying
// diagnosis pin codes and certifying the TEKs.
package verification

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"

	aamodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/go-cmp/cmp"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/errcmp"
	utils "github.com/google/exposure-notifications-server/pkg/verification"
)

func TestVerifyCertificate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name             string
		Warp             time.Duration
		MacAdjustment    string
		MacKeyAdjustment string
		ChangeIssuer     string
		ChangeAudience   string
		Error            string
	}{
		{
			Name: "happy path, valid cert",
		},
		{
			Name:         "bad_issuer",
			ChangeIssuer: "foo",
			Error:        "issuer not found",
		},
		{
			Name:           "bad_audience",
			ChangeAudience: "bar",
			Error:          "audience mismatch for issuer",
		},
		{
			Name:  "past",
			Warp:  -1 * time.Hour,
			Error: "token is expired by",
		},
		{
			Name:  "future",
			Warp:  1 * time.Hour,
			Error: ErrNotValidYet.Error(),
		},
		{
			Name:          "invalid hmac",
			MacAdjustment: "iruinedit",
			Error:         "HMAC mismatch, publish request does not match disgnosis verification certificate",
		},
		{
			Name:             "invalid hmac",
			MacKeyAdjustment: "4",
			Error:            "HMAC mismatch, publish request does not match disgnosis verification certificate",
		},
	}

	// Set up database. Create HealthAuthority + HAKey for the test.
	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	haDB := database.New(testDB)

	for iteration := 0; iteration < 2; iteration++ {
		for version := 0; version <= 1; version++ {
			for _, tc := range cases {
				iteration := iteration
				version := version
				tc := tc

				vname := "v1alpha1"
				if version == 1 {
					vname = "v1"
				}
				mod := "withTR"
				if iteration == 0 {
					mod = "withoutTR"
				}

				t.Run(tc.Name+"_"+vname+"_"+mod, func(t *testing.T) {
					t.Parallel()

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

					issuer := fmt.Sprintf("issuer-%s-%s-%s", vname, mod, t.Name())
					audience := fmt.Sprintf("aud-%s-%s-%s", vname, mod, t.Name())
					healthAuthority := model.HealthAuthority{
						Issuer:   issuer,
						Audience: audience,
						Name:     "Very Real Health Authority",
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

					// Build the verification certificate.
					hmacKeyBytes := make([]byte, 32)
					if _, err := rand.Read(hmacKeyBytes); err != nil {
						t.Fatal(err)
					}
					hmacKeyB64 := base64.StdEncoding.EncodeToString(hmacKeyBytes)

					// Fake authorized app.
					authApp := aamodel.NewAuthorizedApp()
					authApp.AllowedHealthAuthorityIDs[healthAuthority.ID] = struct{}{}

					// Build a sample certificate.
					publish := verifyapi.Publish{
						Keys: []verifyapi.ExposureKey{
							{
								Key:              "IRgYIhYiy4WMl9z68bMk6w==",
								IntervalNumber:   2650032,
								IntervalCount:    144,
								TransmissionRisk: 4,
							},
							{
								Key:              "dPCphLzfG4uzXneNimkPRQ====",
								IntervalNumber:   2650032 + 144,
								IntervalCount:    144,
								TransmissionRisk: 4,
							},
							{
								Key:              "5AUyPfJKcg5cr3OgKdR8Sw==",
								IntervalNumber:   2650032 + 144*2,
								IntervalCount:    144,
								TransmissionRisk: 4,
							},
						},
					}
					// Iteration 0 uses the "alt" hmac.
					if iteration == 0 {
						for i := range publish.Keys {
							publish.Keys[i].TransmissionRisk = 0
						}
					}

					// Calculate the HMAC.
					allHMACs, err := utils.CalculateAllAllowedExposureKeyHMAC(publish.Keys, hmacKeyBytes)
					if err != nil {
						t.Fatal(err)
					}
					hmac := allHMACs[0]
					if iteration == 0 {
						if l := len(allHMACs); l != 2 {
							t.Fatalf("expected 2 hmacs, got: %v", l)
						}
						hmac = allHMACs[1]
					}

					if tc.ChangeIssuer != "" {
						issuer = tc.ChangeIssuer
					}
					if tc.ChangeAudience != "" {
						audience = tc.ChangeAudience
					}

					var claims jwt.Claims
					if version == 0 {
						v1alpha1claims := v1alpha1.NewVerificationClaims()
						v1alpha1claims.Audience = audience
						v1alpha1claims.Issuer = issuer
						v1alpha1claims.IssuedAt = time.Now().Add(tc.Warp).Unix()
						v1alpha1claims.ExpiresAt = time.Now().Add(tc.Warp).Add(5 * time.Minute).Unix()
						v1alpha1claims.SignedMAC = tc.MacAdjustment + base64.StdEncoding.EncodeToString(hmac) // would be generated on the client and passed through.
						v1alpha1claims.ReportType = "confirmed"
						v1alpha1claims.SymptomOnsetInterval = 250250
						// contains legacy transmission risk field, but will be an empty array, just there.
						claims = v1alpha1claims
					} else {
						v1claims := verifyapi.NewVerificationClaims()
						v1claims.Audience = audience
						v1claims.Issuer = issuer
						v1claims.IssuedAt = time.Now().Add(tc.Warp).Unix()
						v1claims.ExpiresAt = time.Now().Add(tc.Warp).Add(5 * time.Minute).Unix()
						v1claims.SignedMAC = tc.MacAdjustment + base64.StdEncoding.EncodeToString(hmac)
						v1claims.ReportType = "confirmed"
						v1claims.SymptomOnsetInterval = 250250
						claims = v1claims
					}

					token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
					token.Header["kid"] = "v1" // matches the key configured above.
					jwtText, err := token.SignedString(privateKey)
					if err != nil {
						t.Fatal(err)
					}

					// Insert this data into the publish request.
					publish.VerificationPayload = jwtText
					publish.HMACKey = tc.MacKeyAdjustment + hmacKeyB64

					// Actually test the verify code.
					verifier, err := New(haDB, &Config{time.Nanosecond, "audience"})
					if err != nil {
						t.Fatal(err)
					}
					verifiedClaims, err := verifier.VerifyDiagnosisCertificate(ctx, authApp, &publish)
					errcmp.MustMatch(t, err, tc.Error)

					if tc.Error == "" {
						if verifiedClaims == nil {
							t.Fatalf("verified claims are nil")
						}

						want := &VerifiedClaims{
							HealthAuthorityID:    healthAuthority.ID,
							ReportType:           "confirmed",
							SymptomOnsetInterval: 250250,
						}
						if diff := cmp.Diff(want, verifiedClaims); diff != "" {
							t.Errorf("claims mismatch (-want, +got):\n%s", diff)
						}
					}
				})
			}
		}
	}
}
