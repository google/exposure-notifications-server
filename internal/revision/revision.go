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

// Package revision defines the internal structure of the revision token
// and utilities for marshal/unmarshal which also encrypts/decrypts the payload.
package revision

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sort"

	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"google.golang.org/protobuf/proto"
)

func buildTokenBufer(eKeys []*model.Exposure) *pb.RevisionTokenData {
	// sort the keys.
	sort.Slice(eKeys, func(i, j int) bool {
		return eKeys[i].ExposureKeyBase64() < eKeys[j].ExposureKeyBase64()
	})
	// Build the protocol buffer version of the revision token data.
	tokenData := pb.RevisionTokenData{
		RevisableKeys: make([]*pb.RevisableKey, 0, len(eKeys)),
	}
	for _, k := range eKeys {
		pbKey := pb.RevisableKey{
			TemporaryExposureKey: make([]byte, len(k.ExposureKey)),
			IntervalNumber:       k.IntervalNumber,
			IntervalCount:        k.IntervalCount,
		}
		copy(pbKey.TemporaryExposureKey, k.ExposureKey)
		tokenData.RevisableKeys = append(tokenData.RevisableKeys, &pbKey)
	}
	return &tokenData
}

// MakeRevisionToken turns the TEK data from a given publish request
// into an encrypted protocol buffer revision token.
func MakeRevisionToken(eKeys []*model.Exposure, aad []byte, kid string, cryptoKey []byte) ([]byte, error) {
	if len(eKeys) == 0 {
		return nil, fmt.Errorf("no keys to build token for")
	}

	tokenData := buildTokenBufer(eKeys)
	plaintext, err := proto.Marshal(tokenData)
	if err != nil {
		return nil, fmt.Errorf("unable to masrhal token data: %w", err)
	}

	// encrypt the serialized proto.
	block, err := aes.NewCipher(cryptoKey)
	if err != nil {
		return nil, fmt.Errorf("bad cipher block: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cipher block: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := aesgcm.Seal(nonce, nonce, plaintext, aad)

	// Build the revision token.
	token := pb.RevisionToken{
		Kid:  kid,
		Data: ciphertext,
	}
	tokenBytes, err := proto.Marshal(&token)
	if err != nil {
		return nil, fmt.Errorf("faield to marshal token: %w", err)
	}

	return tokenBytes, nil
}

// UnmarshalRevisionToken unmarshals a revision token, decrypts the payload,
// and returns the TEK data that was contained in the token if valid.
func UnmarshalRevisionToken(tokenBytes []byte, aad []byte, cryptoKeys map[string][]byte) (*pb.RevisionTokenData, error) {
	var revisionToken pb.RevisionToken
	if err := proto.Unmarshal(tokenBytes, &revisionToken); err != nil {
		return nil, fmt.Errorf("unable to unmarshal proto envelope: %w", err)
	}
	data := revisionToken.Data

	dek, ok := cryptoKeys[revisionToken.Kid]
	if !ok {
		return nil, fmt.Errorf("token has invalid key id: %v", revisionToken.Kid)
	}

	// Decrypt the data block.
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher from dek: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm from dek: %w", err)
	}

	size := aesgcm.NonceSize()
	if len(data) < size {
		return nil, fmt.Errorf("malformed ciphertext")
	}
	nonce, ciphertext := data[:size], data[size:]

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext with dek: %w", err)
	}

	// The plaintext is a pb.RevisionTokenData
	var tokenData pb.RevisionTokenData
	if err := proto.Unmarshal(plaintext, &tokenData); err != nil {
		return nil, fmt.Errorf("faield to unmarshal token data: %w", err)
	}

	return &tokenData, nil
}
