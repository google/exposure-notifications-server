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
	"crypto/hmac"
	"fmt"

	aamodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/base64util"
	pubmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/verification/database"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	utils "github.com/google/exposure-notifications-server/pkg/verification"

	"github.com/dgrijalva/jwt-go"
)

// Verifier can be used to verify public health authority diagnosis verification certificates.
type Verifier struct {
	db *database.HealthAuthorityDB
}

// New creates a new verifier, based on this DB handle.
func New(db *database.HealthAuthorityDB) *Verifier {
	return &Verifier{db}
}

// VerifyDiagnosisCertificate accepts a publish request (from which is extracts the JWT),
// fully verifies the JWT and signture against what the passed in authorrized app is allowed
// to use. Returns any transmission risk overrides if they are present.
func (v *Verifier) VerifyDiagnosisCertificate(ctx context.Context, authApp *aamodel.AuthorizedApp, publish *pubmodel.Publish) (verifyapi.TransmissionRiskVector, error) {
	// These get assigned during the ParseWithClaims closure.
	var healthAuthorityID int64
	var claims *verifyapi.VerificationClaims

	// Unpack JWT so we can determine issuer and key version.
	token, err := jwt.ParseWithClaims(publish.VerificationPayload, &verifyapi.VerificationClaims{}, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodECDSA); !ok || method.Name != jwt.SigningMethodES256.Name {
			return nil, fmt.Errorf("unsupported signing method, must be %v", jwt.SigningMethodES256.Name)
		}

		var ok bool
		claims, ok = token.Claims.(*verifyapi.VerificationClaims)
		if !ok {
			return nil, fmt.Errorf("does not contain expected claim set")
		}

		// Based on issuer, load the key versions.
		ha, err := v.db.GetHealthAuthority(ctx, claims.Issuer)
		if err != nil {
			return nil, fmt.Errorf("error looking up issuer: %v : %w", claims.Issuer, err)
		}

		// Find a key version.
		for _, hak := range ha.Keys {
			if hak.Version == claims.KeyVersion {
				healthAuthorityID = ha.ID
				// Extract the public key from the PEM block.
				return hak.PublicKey()
			}
		}
		return nil, fmt.Errorf("key not found: iss: %v version: %v", claims.Issuer, claims.KeyVersion)
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid verificationPayload")
	}
	if err := claims.Valid(); err != nil {
		return nil, err
	}

	// JWT is valid and signature is valid.
	if _, ok := authApp.AllowedHealthAuthorities[healthAuthorityID]; !ok {
		return nil, fmt.Errorf("app %v has not authorized health authority issuer: %v", authApp.AppPackageName, claims.Issuer)
	}

	// Verify the HMAC.
	jwtHMAC, err := base64util.DecodeString(claims.SignedMAC)
	if err != nil {
		return nil, fmt.Errorf("error decoding HMAC from claims: %w", err)
	}
	secret, err := base64util.DecodeString(publish.HMACKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding HMAC secret from publish request: %w", err)
	}
	wantHMAC, err := utils.CalculateExposureKeyHMAC(publish.Keys, secret)

	if !hmac.Equal(wantHMAC, jwtHMAC) {
		return nil, fmt.Errorf("HMAC mismatch, publish request does not match disgnosis verification certificate")
	}

	// Everything looks good. Return the transmission override vector.
	return claims.TransmissionRisks, nil
}
