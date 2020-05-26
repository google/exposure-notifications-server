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

// package ios manages device attestations using Apple's DeviceCheck API.
package ios

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

var (
	// endpoint is the production Apple DeviceCheck endpoint.
	endpoint = "https://api.devicecheck.apple.com/v1/validate_device_token"

	// The Client's Transport typically has internal state (cached TCP connections), so
	// Clients should be reused instead of created as needed. Clients are safe for concurrent
	// use by multiple goroutines.
	httpClient = &http.Client{
		// the maximum amount of time to wait for a response
		Timeout:   5 * time.Second,
		Transport: &ochttp.Transport{},
	}
)

type VerifyOpts struct {
	KeyID      string
	TeamID     string
	PrivateKey *ecdsa.PrivateKey
}

// ValidateDeviceToken validates the given device token with Apple's servers.
func ValidateDeviceToken(ctx context.Context, deviceToken string, opts *VerifyOpts) error {
	// Verify configuration exists.
	if opts.KeyID == "" {
		return fmt.Errorf("devicecheck: missing key id")
	}
	if opts.TeamID == "" {
		return fmt.Errorf("devicecheck: missing team id")
	}
	if opts.PrivateKey == nil {
		return fmt.Errorf("devicecheck: missing private key")
	}

	// Verify we got a token.
	if deviceToken == "" {
		return fmt.Errorf("devicecheck: missing device token")
	}

	// Generate a JWT.
	signedJwt, err := newSignedJWT(opts.TeamID, opts.KeyID, opts.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to generate jwt: %w", err)
	}

	// Create the JSON body.
	requestBody, err := newValidateRequestBody(deviceToken)
	if err != nil {
		return fmt.Errorf("failed to generated request body: %w", err)
	}

	// Build the request and add the authorization header.
	req, err := http.NewRequest(http.MethodPost, endpoint, requestBody)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", signedJwt))

	// Call Apple's API.
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Return on success.
	if resp.StatusCode == 200 {
		return nil
	}

	// Return error upstream.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy error body (%d): %w", resp.StatusCode, err)
	}
	return fmt.Errorf("failed to attest: (%d) %s", resp.StatusCode, body)
}

// ParsePrivateKey parses a PEM-encoded .p8 from the Apple Device Portal.
func ParsePrivateKey(s string) (*ecdsa.PrivateKey, error) {
	// Decode the pem.
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	// Parse it as PKCS8. According to docs, Apple keys are always PKCS8.
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Type-assert to ecdsa.
	ecKey, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is %T, want ecdsa", parsedKey)
	}

	return ecKey, nil
}

// newSignedJWT creates a new signed JWT suitable for authenticating to Apple's
// servers.
func newSignedJWT(teamID, keyID string, privateKey *ecdsa.PrivateKey) (string, error) {
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": teamID,
		"iat": time.Now().UTC().Unix(),
	})
	jwtToken.Header["alg"] = "ES256"
	jwtToken.Header["kid"] = keyID

	return jwtToken.SignedString(privateKey)
}

// newValidateRequestBody creates a request body used for devicecheck.
func newValidateRequestBody(deviceToken string) (io.Reader, error) {
	i := &validateRequest{
		DeviceToken:   deviceToken,
		TransactionID: uuid.New().String(),
		Timestamp:     time.Now().UTC().UnixNano() / int64(time.Millisecond),
	}
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(i); err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return &b, nil
}

type validateRequest struct {
	// DeviceToken is the provided iOS device token.
	DeviceToken string `json:"device_token"`

	// TransactionID is a randomly-generated UUID.
	TransactionID string `json:"transaction_id"`

	// Timestamp is the current UNIX timestamp in _milliseconds_.
	Timestamp int64 `json:"timestamp"`
}
