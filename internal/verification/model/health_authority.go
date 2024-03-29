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

// Package model is a model abstraction of health authorities.
package model

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

// HealthAuthority represents a public health authority that is authorized to
// issue diagnosis verification certificates accepted by this server.
type HealthAuthority struct {
	ID             int64
	Issuer         string
	Audience       string
	Name           string
	Keys           []*HealthAuthorityKey
	JwksURI        *string
	EnableStatsAPI bool
}

// JWKSEnabled returns true if JWKS discovery is enabled for this health authority.
func (ha *HealthAuthority) JWKSEnabled() bool {
	return !(ha.JwksURI == nil || len(*ha.JwksURI) == 0)
}

// SetJWKS sets the optional JwksURI property of the HealthAuthority.
func (ha *HealthAuthority) SetJWKS(uri string) {
	uri = project.TrimSpaceAndNonPrintable(uri)
	if uri == "" {
		ha.JwksURI = nil
		return
	}
	ha.JwksURI = &uri
}

// Validate returns an error if the HealthAuthority struct is not valid.
func (ha *HealthAuthority) Validate() error {
	if ha.Issuer == "" {
		return errors.New("issuer cannot be empty")
	}
	if ha.Audience == "" {
		return errors.New("audience cannot be empty")
	}
	if ha.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

// HealthAuthorityKey represents a public key version for a given health authority.
type HealthAuthorityKey struct {
	AuthorityID  int64
	Version      string
	From         time.Time
	Thru         time.Time
	PublicKeyPEM string
}

// Validate returns an error if the HealthAuthorityKey is not valid.
func (k *HealthAuthorityKey) Validate() error {
	if _, err := k.PublicKey(); err != nil {
		return fmt.Errorf("invalid public key PEM block: %w", err)
	}
	return nil
}

func (k *HealthAuthorityKey) IsFuture() bool {
	return k.From.After(time.Now())
}

// IsValid returns true if the key is valid based on the current time.
func (k *HealthAuthorityKey) IsValid() bool {
	return k.IsValidAt(time.Now())
}

// IsValidAt returns true if the key is valid at a specific point in time.
func (k *HealthAuthorityKey) IsValidAt(t time.Time) bool {
	return t.After(k.From) && (k.Thru.IsZero() || k.Thru.After(t))
}

// Revoke revokes a key.
func (k *HealthAuthorityKey) Revoke() {
	k.Thru = time.Now().UTC()
	if !k.Thru.After(k.From) {
		// make it so that the key doesn't expire before it is active.
		k.Thru = k.From
	}
}

// PublicKey decodes the PublicKeyPEM text and returns the `*ecdsa.PublicKey`
// This system only supports verifying ECDSA JWTs, `alg: ES256`.
func (k *HealthAuthorityKey) PublicKey() (*ecdsa.PublicKey, error) {
	return keys.ParseECDSAPublicKey(k.PublicKeyPEM)
}
