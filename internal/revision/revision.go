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
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision/database"
	"google.golang.org/protobuf/proto"
)

// TokenManager is responsible for creating and unlocking revision tokens.
type TokenManager struct {
	db *database.RevisionDB

	mu        sync.RWMutex
	allowed   map[int64]*database.RevisionKey
	effective *database.RevisionKey

	cacheDuration     time.Duration
	cacheRefreshAfter time.Time
}

// New creates a new TokenManager that uses a database handle to manage a cache
// of allowed revision keys.
func New(ctx context.Context, db *database.RevisionDB, cacheDuration time.Duration) (*TokenManager, error) {
	now := time.Now()
	tm := &TokenManager{
		db:                db,
		allowed:           make(map[int64]*database.RevisionKey),
		cacheDuration:     cacheDuration,
		cacheRefreshAfter: now.Add(-2 * cacheDuration),
	}
	if err := tm.maybeRefreshCache(ctx); err != nil {
		return nil, err
	}
	return tm, nil
}

func (tm *TokenManager) expired() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return time.Now().After(tm.cacheRefreshAfter)
}

func (tm *TokenManager) maybeRefreshCache(ctx context.Context) error {
	if !tm.expired() {
		return nil
	}
	// Escalate to a write lock and refresh the cache.
	tm.mu.Lock()
	defer tm.mu.Unlock()

	reload, err := tm.isReloadNeeded(ctx)
	if err != nil {
		return fmt.Errorf("unable to read revsion keys: %w", err)
	}
	if !reload {
		return nil
	}

	// At this point reload is needed. Trash the
	tm.allowed = make(map[int64]*database.RevisionKey)
	tm.effective = nil

	// Go back and reload and unwrap the effective keys.
	logger := logging.FromContext(ctx)
	logger.Info("reloading revision key cache")

	effectiveID, allowed, err := tm.db.GetAllowedRevisionKeys(ctx)
	if err != nil {
		logger.Errorf("unable to read revision keys: %v", err)
		return fmt.Errorf("reading revision key cache: %w", err)
	}

	if len(allowed) == 0 {
		return fmt.Errorf("no revision keys exist")
	}

	for _, rk := range allowed {
		tm.allowed[rk.KeyID] = rk
	}
	tm.effective = tm.allowed[effectiveID]
	// we did it! mark the next refresh time.
	tm.cacheRefreshAfter = time.Now().Add(tm.cacheDuration)
	return nil
}

// Determine if we actually need to reload and unwrap keys.
// Must be called under write lock.
func (tm *TokenManager) isReloadNeeded(ctx context.Context) (bool, error) {
	// If there is no effective key, reload.
	if tm.effective == nil {
		return true, nil
	}

	effectiveID, allowedIDs, err := tm.db.GetAllowedRevisionKeyIDs(ctx)
	if err != nil {
		return true, err
	}
	if effectiveID != tm.effective.KeyID {
		return true, nil
	}

	for k := range allowedIDs {
		if rk := tm.allowed[k]; rk == nil {
			// Found an allowed key that we haven't seen yet.
			return true, nil
		}
	}
	// remove any keys that are no longer allowed from the cache.
	for k := range tm.allowed {
		if _, ok := allowedIDs[k]; !ok {
			delete(tm.allowed, k)
		}
	}

	return false, nil
}

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
// This is using envelope encryption, based on the currently active revision key.
func (tm *TokenManager) MakeRevisionToken(ctx context.Context, eKeys []*model.Exposure, aad []byte) ([]byte, error) {
	if len(eKeys) == 0 {
		return nil, fmt.Errorf("no keys to build token for")
	}

	if err := tm.maybeRefreshCache(ctx); err != nil {
		return nil, err
	}
	// Capture DEK and KID in read lock, but don't do encryption with the lock
	var dek []byte
	var kid int64
	{
		tm.mu.RLock()
		dek = tm.effective.DEK
		kid = tm.effective.KeyID
		tm.mu.RUnlock()
	}

	tokenData := buildTokenBufer(eKeys)
	plaintext, err := proto.Marshal(tokenData)
	if err != nil {
		return nil, fmt.Errorf("unable to masrhal token data: %w", err)
	}

	// encrypt the serialized proto.
	block, err := aes.NewCipher(dek)
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
//
// The incoming key ID is used to determine if this token can still be unlocked.
func (tm *TokenManager) UnmarshalRevisionToken(ctx context.Context, tokenBytes []byte, aad []byte) (*pb.RevisionTokenData, error) {
	if err := tm.maybeRefreshCache(ctx); err != nil {
		return nil, err
	}

	var revisionToken pb.RevisionToken
	if err := proto.Unmarshal(tokenBytes, &revisionToken); err != nil {
		return nil, fmt.Errorf("unable to unmarshal proto envelope: %w", err)
	}
	data := revisionToken.Data

	var dek []byte
	// Capture the DEK under read lock, but don't hold lock for decryption.
	{
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		rk, ok := tm.allowed[revisionToken.Kid]
		if !ok {
			return nil, fmt.Errorf("token has invalid key id: %v", revisionToken.Kid)
		}
		dek = rk.DEK
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
