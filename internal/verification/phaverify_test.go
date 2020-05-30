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

// Package verification provides the ability to verify the diagnosis certificates
// (JWTs) coming from public health authorities that are responsible for verifying
// diagnosis pin codes and ceritfying the TEKs.
package verification

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

	aamodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	coredb "github.com/google/exposure-notifications-server/internal/database"
	pubmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	utils "github.com/google/exposure-notifications-server/pkg/verification"
)

// TODO(mikehelmick) : this only tests the happy path. Write non happy tests.
func TestVerifyCertificate(t *testing.T) {
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

	// Set up database. Create HealthAuthority + HAKey for the test.
	testDB := coredb.NewTestDatabase(t)
	ctx := context.Background()
	haDB := database.New(testDB)

	healthAuthority := model.HealthAuthority{
		Issuer:   "doh.my.gov",
		Audience: "exposure-notifications-server",
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
	publish := pubmodel.Publish{
		Keys: []pubmodel.ExposureKey{
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

	// Calculate the HMAC.
	hmac, err := utils.CalculateExposureKeyHMAC(publish.Keys, hmacKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	claims := verifyapi.NewVerificationClaims()
	claims.Audience = "exposure-notifications-server"
	claims.Issuer = "doh.my.gov"
	claims.IssuedAt = time.Now().UTC().Unix()
	claims.ExpiresAt = time.Now().UTC().Add(5 * time.Minute).Unix()
	claims.SignedMAC = base64.StdEncoding.EncodeToString(hmac) // would be generated on the client and passed through.
	claims.KeyVersion = "v1"                                   // matches the key configured above.
	// leaves PHAClaims and transmission risk overrides out of it.

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	jwtText, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	// Insert this data into the publish request.
	publish.VerificationPayload = jwtText
	publish.HMACKey = hmacKeyB64

	// Actually test the verify code.
	verifier := New(haDB)
	overrides, err := verifier.VerifyDiagnosisCertificate(ctx, authApp, &publish)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Errorf("wanted no overrides, got %v", overrides)
	}
}
