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

package model

import (
	"crypto/ecdsa"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
)

// ImportFilePublicKey represents a possible signing key
// for export files being imported into this system.
// A given ExportImportID can have more than one associated key,
// and more than one that is currently valid.
type ImportFilePublicKey struct {
	ExportImportID int64
	KeyID          string
	KeyVersion     string
	PublicKeyPEM   string
	From           time.Time
	Thru           *time.Time
}

func (pk *ImportFilePublicKey) PublicKey() (*ecdsa.PublicKey, error) {
	return keys.ParseECDSAPublicKey(pk.PublicKeyPEM)
}

func (pk *ImportFilePublicKey) Revoke() {
	now := time.Now().UTC()
	pk.Thru = &now
}

func (pk *ImportFilePublicKey) Active() bool {
	now := time.Now().UTC()
	return pk.From.Before(now) && (pk.Thru == nil || now.Before(*pk.Thru))
}

func (pk *ImportFilePublicKey) Future() bool {
	now := time.Now().UTC()
	return now.Before(pk.From)
}
