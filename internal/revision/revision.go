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
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"google.golang.org/protobuf/proto"
)

var (
	// Used for padding only.
	zeroTEK = pb.RevisableKey{
		TemporaryExposureKey: make([]byte, 16),
		IntervalCount:        0,
		IntervalNumber:       0,
	}
)

// TokenManager is responsible for creating and unlocking revision tokens.
type TokenManager struct {
	db *database.RevisionDB

	// All encrypt/decrypt operations are done under read lock.
	// Cache refresh escalates to a write lock.
	mu sync.RWMutex

	// A store of the currently allowed revision keys for decryption purposes.
	allowed map[int64]*database.RevisionKey
	// A pointers to the currently active key for encryption purposes.
	effective *database.RevisionKey

	// Pads tokens so that the size of the token can't be used to determine how many keys
	// are held within.
	minTokenSize int

	// The allowed/effective keys are cached to avoid excessive decrypt calls to the KMS system to unwrap
	// the individual revision keys.
	// A cache refresh is initially a shallow refresh, if the IDs of allowed/effective keys haven't changed,
	// we don't re-unwrap the keys. If there are any changes to the IDs, all of the data is reloaded and
	// the keys are unwrapped.
	cacheDuration     time.Duration
	cacheRefreshAfter time.Time
}

// New creates a new TokenManager that uses a database handle to manage a cache
// of allowed revision keys.
func New(ctx context.Context, db *database.RevisionDB, cacheDuration time.Duration, minTokenSize uint) (*TokenManager, error) {
	if cacheDuration > 60*time.Minute {
		return nil, fmt.Errorf("cache duration must be <= 60 minutes, got: %v", cacheDuration)
	}
	now := time.Now()
	tm := &TokenManager{
		db:                db,
		allowed:           make(map[int64]*database.RevisionKey),
		minTokenSize:      int(minTokenSize),
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

	// At this point reload is needed. Trash the information currently stored
	// so that if something fails on refresh, the cache has been invalidated.
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

	// To aid in system upgrade, assuming env vars are setup correctly,
	// we autocreate the first wrapped revision key.
	if len(allowed) == 0 {
		logger.Errorf("no revision keys exist - creating one.")
		rk, err := tm.db.CreateRevisionKey(ctx)
		if err != nil {
			return fmt.Errorf("unable to bootstrap revision keys: %w", err)
		}
		allowed = append(allowed, rk)
		effectiveID = rk.KeyID
	}

	for _, rk := range allowed {
		tm.allowed[rk.KeyID] = rk
	}
	tm.effective = tm.allowed[effectiveID]
	// We did it! mark the next refresh time.
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

func buildTokenBufer(previous *pb.RevisionTokenData, eKeys []*model.Exposure) *pb.RevisionTokenData {
	// Build the protocol buffer version of the revision token data.
	tokenData := pb.RevisionTokenData{
		RevisableKeys: make([]*pb.RevisableKey, 0, len(eKeys)),
	}
	got := make(map[string]struct{})

	// Add in previous keys that weren't also in the new exposures. This needs to
	// come first so the revision token is valid for all keys, not just the ones
	// uploaded now.
	if previous != nil {
		for _, rk := range previous.RevisableKeys {
			got[base64.StdEncoding.EncodeToString(rk.TemporaryExposureKey)] = struct{}{}
			tokenData.RevisableKeys = append(tokenData.RevisableKeys, rk)
		}
	}

	// Now add new keys and their metadata, iff they aren't already in the list.
	for _, k := range eKeys {
		if _, ok := got[k.ExposureKeyBase64()]; !ok {
			tokenData.RevisableKeys = append(tokenData.RevisableKeys, &pb.RevisableKey{
				TemporaryExposureKey: append([]byte{}, k.ExposureKey...), // deep copy
				IntervalNumber:       k.IntervalNumber,
				IntervalCount:        k.IntervalCount,
			})
		}
	}

	return &tokenData
}

// MakeRevisionToken turns the TEK data from a given publish request
// into an encrypted protocol buffer revision token.
// This is using envelope encryption, based on the currently active revision key.
func (tm *TokenManager) MakeRevisionToken(ctx context.Context, previous *pb.RevisionTokenData, eKeys []*model.Exposure, aad []byte) ([]byte, error) {
	if len(eKeys) == 0 {
		return nil, fmt.Errorf("no keys to build token for")
	}

	if err := tm.maybeRefreshCache(ctx); err != nil {
		return nil, err
	}
	// Capture DEK and KID in read lock, but don't do encryption with the lock
	var dek []byte
	var kid string
	{
		tm.mu.RLock()
		dek = tm.effective.DEK
		kid = tm.effective.KeyIDString()
		tm.mu.RUnlock()
	}

	tokenData := buildTokenBufer(previous, eKeys)
	// Padd the revisable keys out w/ the zero key.
	for len(tokenData.RevisableKeys) < tm.minTokenSize {
		tokenData.RevisableKeys = append(tokenData.RevisableKeys, &zeroTEK)
	}

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
	kid, err := strconv.ParseInt(revisionToken.Kid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid key id: %w", err)
	}

	var dek []byte
	// Capture the DEK under read lock, but don't hold lock for decryption.
	{
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		rk, ok := tm.allowed[kid]
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
	var paddedTokenData pb.RevisionTokenData
	if err := proto.Unmarshal(plaintext, &paddedTokenData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	var tokenData pb.RevisionTokenData
	for _, rk := range paddedTokenData.RevisableKeys {
		if rk.IntervalNumber == 0 && rk.IntervalCount == 0 {
			continue
		}
		tokenData.RevisableKeys = append(tokenData.RevisableKeys, rk)
	}

	return &tokenData, nil
}
