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
// diagnosis pin codes and certifying the TEKs.
package verification

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"

	aamodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"
	utils "github.com/google/exposure-notifications-server/pkg/verification"

	"github.com/dgrijalva/jwt-go"
)

var (
	// ErrNoPublicKeys indicates no public keys were found when verifying the certificate.
	ErrNoPublicKeys = errors.New("no active public keys for health authority")
	ErrNotValidYet  = errors.New("not valid yet (NBF or IAT) in the future")
)

// Verifier can be used to verify public health authority diagnosis verification certificates.
type Verifier struct {
	db      *database.HealthAuthorityDB
	config  *Config
	haCache *cache.Cache
}

// New creates a new verifier, based on this DB handle.
func New(db *database.HealthAuthorityDB, config *Config) (*Verifier, error) {
	cache, err := cache.New(config.CacheDuration)
	if err != nil {
		return nil, err
	}
	return &Verifier{db, config, cache}, nil
}

// VerifiedClaims represents the relevant claims extracted from a verified
// certificate that may need to be applied.
type VerifiedClaims struct {
	HealthAuthorityID    int64
	ReportType           string // blank indicates no report type was present.
	SymptomOnsetInterval uint32 // 0 indicates no symptom onset interval present. This should be checked for "reasonable" value before application.
}

// VerifyDiagnosisCertificate accepts a publish request (from which is extracts the JWT),
// fully verifies the JWT and signture against what the passed in authorrized app is allowed
// to use. Returns any transmission risk overrides if they are present.
func (v *Verifier) VerifyDiagnosisCertificate(ctx context.Context, authApp *aamodel.AuthorizedApp, publish *verifyapi.Publish) (*VerifiedClaims, error) {
	logger := logging.FromContext(ctx)
	// These get assigned during the ParseWithClaims closure.
	var healthAuthorityID int64
	var claims *verifyapi.VerificationClaims

	// Unpack JWT so we can determine issuer and key version.
	// ParseWithClaims also calls .Valid() on the parsed token.
	token, err := jwt.ParseWithClaims(publish.VerificationPayload, &verifyapi.VerificationClaims{}, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodECDSA); !ok || method.Name != jwt.SigningMethodES256.Name {
			return nil, fmt.Errorf("unsupported signing method, must be %v", jwt.SigningMethodES256.Name)
		}

		var ok bool
		kid, ok := token.Header[verifyapi.KeyIDHeader]
		if !ok {
			return nil, fmt.Errorf("missing required header field, 'kid' indicating key id")
		}

		claims, ok = token.Claims.(*verifyapi.VerificationClaims)
		if !ok {
			return nil, fmt.Errorf("does not contain expected claim set")
		}

		lookup := func() (interface{}, error) {
			// Based on issuer, load the key versions.
			ha, err := v.db.GetHealthAuthority(ctx, claims.Issuer)
			// Special case not found so that we can cache it.
			if errors.Is(err, database.ErrHealthAuthorityNotFound) {
				logger.Warnw("requested issuer not found", "iss", claims.Issuer)
				return nil, nil
			}
			if err != nil {
				return nil, fmt.Errorf("error looking up issuer: %v : %w", claims.Issuer, err)
			}
			return ha, nil
		}
		cacheVal, err := v.haCache.WriteThruLookup(claims.Issuer, lookup)
		if err != nil {
			return nil, err
		}

		if cacheVal == nil {
			return nil, fmt.Errorf("issuer not found: %v", claims.Issuer)
		}

		ha := cacheVal.(*model.HealthAuthority)

		// Advisory check the aud.
		if claims.Audience != ha.Audience {
			return nil, fmt.Errorf("audience mismatch for issuer: %v (+%s, -%s)", ha.Issuer, claims.Audience, ha.Audience)
		}

		// Find a key version.
		for _, hak := range ha.Keys {
			// Key version matches and the key is valid based on the current time.
			if hak.Version == kid && hak.IsValid() {
				healthAuthorityID = ha.ID
				// Extract the public key from the PEM block.
				return hak.PublicKey()
			}
		}
		return nil, ErrNoPublicKeys
	})
	if err != nil {
		// Check for specific errors in the bitmask that may exist and
		// convert them to application local errors.
		validationError := new(jwt.ValidationError)
		if errors.As(err, &validationError) {
			if mask := validationError.Errors; mask&jwt.ValidationErrorIssuedAt != 0 || mask&jwt.ValidationErrorNotValidYet != 0 {
				return nil, ErrNotValidYet
			}
		}
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid verificationPayload")
	}

	// JWT is valid and signature is valid.
	// This is chacked after the signature verification to prevent timing attacks.
	if _, ok := authApp.AllowedHealthAuthorityIDs[healthAuthorityID]; !ok {
		return nil, fmt.Errorf("app %v has not authorized health authority issuer: %v", authApp.AppPackageName, claims.Issuer)
	}

	// Verify our cutom claim types
	if err := claims.CustomClaimsValid(); err != nil {
		return nil, err
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
	// Allow the HMAC to be calculated without transmission risk values IFF all transmission risks are zero.
	validHMACs, err := utils.CalculateAllAllowedExposureKeyHMAC(publish.Keys, secret)
	if err != nil {
		return nil, fmt.Errorf("calculating expected HMAC: %w", err)
	}

	valid := false
	for _, wantHMAC := range validHMACs {
		valid = valid || hmac.Equal(wantHMAC, jwtHMAC)
	}
	if !valid {
		return nil, fmt.Errorf("HMAC mismatch, publish request does not match disgnosis verification certificate")
	}

	// Everything looks good. Return the relevant verified claims.
	return &VerifiedClaims{
		HealthAuthorityID:    healthAuthorityID,
		ReportType:           claims.ReportType,
		SymptomOnsetInterval: claims.SymptomOnsetInterval,
	}, nil
}
