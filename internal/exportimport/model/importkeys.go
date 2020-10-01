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

package model

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

type ImportFilePublicKey struct {
	ExportImportID int64
	KeyID          string
	KeyVersion     string
	PublicKeyPEM   string
	From           time.Time
	Thru           *time.Time
}

func (pk *ImportFilePublicKey) PublicKey() (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pk.PublicKeyPEM))
	if block == nil {
		return nil, errors.New("unable to decode PEM block containing PUBLIC KEY")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509.ParsePKIXPublicKey: %w", err)
	}

	switch typ := pub.(type) {
	case *ecdsa.PublicKey:
		return typ, nil
	default:
		return nil, fmt.Errorf("unsupported public key type: %T", typ)
	}
}
