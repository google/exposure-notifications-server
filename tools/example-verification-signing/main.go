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

// This package implements a sample server that implements a piece of the public health authority
// verification protocol: https://github.com/google/exposure-notifications-server/blob/main/docs/design/verification_protocol.md
//
// To call this server using curl:
// curl -d '{"verificationCode":"fakeCode","tekhmac":"replace w/ tek hmac"}' -H "Content-Type: application/json" -X POST http://localhost:8080/
//
// The FULL protocol is implemented by
// https://github.com/google/exposure-notifications-verification-server/
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/keys"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// VerifyRequest is the JSON structure the server accepts in order to issue a certificate.
// This is a simplified example and doesn't represent the full capabilities.
type VerifyRequest struct {
	VerificationCode string `json:"verificationCode"`
	HMAC             string `json:"tekhmac"`
}

// VerifyResponse is the response API type.
type VerifyResponse struct {
	Error                   string `json:"error"`
	VerificationCertificate string `json:"verificationCertificate"`
}

type config struct {
	KeyManager keys.Config

	SigningKey string `env:"SIGNING_KEY,required"`
	KeyVersion string `env:"KEY_VERSION, default=1"`

	// Standard claims for the certificate.
	Audience      string        `env:"AUDIENCE, default=exposure-notifications-service"`
	Issuer        string        `env:"ISSUER, default=Department of Health"`
	ValidDuration time.Duration `env:"VALID_DURATION, default=5m"`
}

func (c *config) KeyManagerConfig() *keys.Config {
	return &c.KeyManager
}

func main() {
	oneDay := 24 * time.Hour

	ctx := context.Background()
	var cfg config
	env, err := setup.Setup(ctx, &cfg)
	if err != nil {
		log.Fatalf("setup.Setup: %v", err)
	}
	defer env.Close(ctx)

	router := gin.Default()
	signer, err := env.GetSignerForKey(ctx, cfg.SigningKey)
	if err != nil {
		log.Fatalf("unable to retrieve signing key %v, error: %v", cfg.SigningKey, err)
	}

	router.POST("/", func(c *gin.Context) {
		// Parse the VerifyRequest
		var request VerifyRequest
		var response VerifyResponse
		if err := c.ShouldBindJSON(&request); err != nil {
			response.Error = err.Error()
			c.JSON(http.StatusBadRequest, response)
			return
		}

		now := time.Now().UTC()

		// Here - we simply sign the claims and do not to any code or token verification.
		// The full verification flow reference implementation is available at:
		// https://github.com/google/exposure-notifications-verification-server/

		// Build a JWT that contains the Standard and Extended claims as defined in
		// pkg/api/v1alpha1/verification_types.go
		claims := v1alpha1.NewVerificationClaims()

		// Here, we show an example of a confirmed lab test, conducted yesterday (-1 day),
		// with symptom onset occurring 2 days before the test (-3 days from now).
		claims.ReportType = v1alpha1.ReportTypeConfirmed
		claims.TestDateInterval = uint32(model.IntervalNumber(now.Add(-1 * oneDay).Truncate(oneDay)))
		claims.SymptomOnsetInterval = uint32(model.IntervalNumber(now.Add(-3 * oneDay).Truncate(oneDay)))

		// optionally add transmission risks
		claims.SignedMAC = request.HMAC
		// Add in the standard claims.
		claims.StandardClaims.Audience = cfg.Audience
		claims.StandardClaims.Issuer = cfg.Issuer
		claims.StandardClaims.IssuedAt = now.Unix()
		claims.StandardClaims.ExpiresAt = now.Add(cfg.ValidDuration).Unix()
		claims.StandardClaims.NotBefore = now.Add(-1 * time.Second).Unix()

		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		// The key version goes in the JWT header, not the claims.
		token.Header[v1alpha1.KeyIDHeader] = cfg.KeyVersion

		signingString, err := token.SigningString()
		if err != nil {
			response.Error = err.Error()
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		digest := sha256.Sum256([]byte(signingString))
		sig, err := signer.Sign(rand.Reader, digest[:], nil)
		if err != nil {
			response.Error = fmt.Sprintf("error signing JWT: %v", err)
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// Unpack the ASN1 signature. ECDSA signers are supposed to return this format
		// https://golang.org/pkg/crypto/#Signer
		// All suporrted signers in thise codebase are verified to return ASN1.
		var parsedSig struct{ R, S *big.Int }
		// ASN1 is not the expected format for an ES256 JWT signature.
		// The output format is specified here, https://tools.ietf.org/html/rfc7518#section-3.4
		// Reproduced here for reference.
		//    The ECDSA P-256 SHA-256 digital signature is generated as follows:
		//
		// 1 .  Generate a digital signature of the JWS Signing Input using ECDSA
		//      P-256 SHA-256 with the desired private key.  The output will be
		//      the pair (R, S), where R and S are 256-bit unsigned integers.
		_, err = asn1.Unmarshal(sig, &parsedSig)
		if err != nil {
			response.Error = fmt.Sprintf("unable to parse JWT signature: %v", err)
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		keyBytes := 256 / 8
		if 256%8 > 0 {
			keyBytes++
		}

		// 2. Turn R and S into octet sequences in big-endian order, with each
		// 		array being be 32 octets long.  The octet sequence
		// 		representations MUST NOT be shortened to omit any leading zero
		// 		octets contained in the values.
		rBytes := parsedSig.R.Bytes()
		rBytesPadded := make([]byte, keyBytes)
		copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

		sBytes := parsedSig.S.Bytes()
		sBytesPadded := make([]byte, keyBytes)
		copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

		// 3. Concatenate the two octet sequences in the order R and then S.
		//	 	(Note that many ECDSA implementations will directly produce this
		//	 	concatenation as their output.)
		sig = append(rBytesPadded, sBytesPadded...)

		// 4.  The resulting 64-octet sequence is the JWS Signature value.
		response.VerificationCertificate = strings.Join([]string{signingString, jwt.EncodeSegment(sig)}, ".")
		c.JSON(http.StatusOK, response)
	})

	router.Run()
}
