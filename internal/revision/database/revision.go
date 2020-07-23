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

// Package database contains the management of interactions with the database
// for createion and storage of the wrapped keys that encrypet revision certificates.
//
// RevisionKey data is stored in the revisionkeys table.
// * The most recently created 'allowed' key is considerd to be the effective key.
//   The effective key is used to encrypt outgoing revision tokens.
// * Any still 'allowed' key can be used to decrypt incoming revision tokens.
//
// This package also supports the creation of new keys with a locally generated
// AES key that is encrypted using the provided KMS and stored in the database
// in it's encrypted form.
package database

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/jackc/pgx/v4"
)

// RevisionKey represents an individual revision key.
type RevisionKey struct {
	KeyID         int64
	AAD           []byte // AAD for the wrapping/unwrapping of the cipher block.
	WrappedCipher []byte
	CreatedAt     time.Time
	Allowed       bool

	// The unwrapped cipher.
	DEK []byte
}

// KeyIDString returns the keyID as a string that can be used in the encoded revision tokens.
func (r *RevisionKey) KeyIDString() string {
	return strconv.FormatInt(r.KeyID, 10)
}

// KMSConfig represents the configuration of the RevisionDB in terms how how it should utilize the
// given KeyManager for wrapping/unwrapping keys.
type KMSConfig struct {
	WrapperKeyID string
	WrapperAAD   []byte
	KeyManager   keys.KeyManager
}

// Copy creates a deep copy of the KMSConfig
// This is done to ensure that we have a clean copy of the ADD slice that can't be changed externally.
func (k *KMSConfig) Copy() *KMSConfig {
	c := KMSConfig{
		WrapperKeyID: k.WrapperKeyID,
		WrapperAAD:   make([]byte, 0, len(k.WrapperAAD)),
		KeyManager:   k.KeyManager,
	}
	copy(c.WrapperAAD, k.WrapperAAD)
	return &c
}

// RevisionDB wraps a database connection and provides functions for interacting with revision keys.
type RevisionDB struct {
	db     *database.DB
	config *KMSConfig
}

// New creates a new `RevisionDB`
func New(db *database.DB, c *KMSConfig) (*RevisionDB, error) {
	if c.WrapperKeyID == "" {
		return nil, fmt.Errorf("no KMS key ID passed in to revision.New")
	}
	if len(c.WrapperAAD) == 0 {
		return nil, fmt.Errorf("no AAD provided")
	}
	if c.KeyManager == nil {
		return nil, fmt.Errorf("no KeyManager provided")
	}
	return &RevisionDB{
		db:     db,
		config: c.Copy(),
	}, nil
}

// DestroyKey zeros out the wrapped key and marks the key as allowed=false.
func (rdb *RevisionDB) DestroyKey(ctx context.Context, keyID int64) error {
	logger := logging.FromContext(ctx)
	logger.Warnf("destroying key material for revision key ID %v", keyID)

	if err := rdb.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE
				RevisionKeys
			SET
				wrapped_cipher = $1, aad = $2, allowed = $3
			WHERE
				kid = $4
		`, nil, nil, false, keyID)
		if err != nil {
			return fmt.Errorf("updating revisionkey: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("revision key was not updated as expected")
		}
		return nil
	}); err != nil {
		logger.Errorf("failed to destroy revision kid: %v: %v", keyID, err)
	}
	return nil
}

// GetAllowedRevisionKeyIDs returns just the IDs of still allowed keys.
// Once the keys have been unwrapped, there is no reason to continue to unwrap them.
//
// The first return value is the ID of the currently effective key (most recently created, still active)
// The second return value is a map of the currently allowed keys for decryption
func (rdb *RevisionDB) GetAllowedRevisionKeyIDs(ctx context.Context) (int64, map[int64]struct{}, error) {
	var effectiveID int64
	keys := make(map[int64]struct{})
	if err := rdb.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Ordering by created_at DESC puts the "effective key" first.
		rows, err := tx.Query(ctx, `
			SELECT
				kid
			FROM
				revisionkeys
			WHERE
				allowed=$1
			ORDER BY created_at DESC`, true)
		if err != nil {
			return nil
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			keys[id] = struct{}{}
			// The first key we see (due to sort) is the effective key.
			if effectiveID == 0 {
				effectiveID = id
			}
		}

		return nil
	}); err != nil {
		return 0, nil, fmt.Errorf("unable to read keys: %w1", err)
	}
	return effectiveID, keys, nil
}

// GetAllowedRevisionKeys returns all of the curently allowed revision keys.
// This method will unwrap all of the keys so that they can be used to create and verify
// revision tokens.
//
// The first return value is the ID of the effective RevisionKey
// The second is a slice of all currently allowed RevisionKeys for decryption purposes. The returned
// revision keys will be sorted in reverse time order by creation time.
func (rdb *RevisionDB) GetAllowedRevisionKeys(ctx context.Context) (int64, []*RevisionKey, error) {
	logger := logging.FromContext(ctx)
	var effectiveID int64
	keys := make([]*RevisionKey, 0)
	if err := rdb.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Need to sort by created_at DESC so the first key encountered is the "effective" key.
		rows, err := tx.Query(ctx, `
			SELECT
				kid, aad, wrapped_cipher, created_at, allowed
			FROM
				revisionkeys
			WHERE
				allowed=$1
			ORDER BY created_at DESC`, true)
		if err != nil {
			return nil
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}
			var r RevisionKey
			if err := rows.Scan(&r.KeyID, &r.AAD, &r.WrappedCipher, &r.CreatedAt, &r.Allowed); err != nil {
				return err
			}
			keys = append(keys, &r)
			// The effective KEY is the first one due to ORDER BY clause.
			if effectiveID == 0 {
				effectiveID = r.KeyID
			}
		}

		return nil
	}); err != nil {
		return 0, nil, fmt.Errorf("unable to read keys: %w1", err)
	}

	unwrappedKeys := make([]*RevisionKey, 0, len(keys))
	// Attempt to unwrap all of the keys
	for _, wk := range keys {
		unwrapped, err := rdb.decrypt(ctx, wk.WrappedCipher)
		if err != nil {
			logger.Errorf("still allowed revision key that can't be unwrapped: kid: %v error: %v", wk.KeyID, err)
			return 0, nil, fmt.Errorf("unable to unwrap revision key: %w", err)
		}
		wk.DEK = unwrapped
		unwrappedKeys = append(unwrappedKeys, wk)
	}

	// Go through an unwrap all of the keys.
	return effectiveID, unwrappedKeys, nil
}

// GetEffectiveRevisionKey returnes the revision key to use when encrypting revision tokens.
// This is consided the most recently created key that is still "allowed"
func (rdb *RevisionDB) GetEffectiveRevisionKey(ctx context.Context) (*RevisionKey, error) {
	var revKey *RevisionKey
	if err := rdb.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				kid, aad, wrapped_cipher, created_at, allowed
			FROM
				revisionkeys
			WHERE
				allowed=$1
			ORDER BY created_at DESC
			LIMIT 1`, true)
		if err != nil {
			return err
		}
		defer rows.Close()

		if rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}
			var r RevisionKey
			if err := rows.Scan(&r.KeyID, &r.AAD, &r.WrappedCipher, &r.CreatedAt, &r.Allowed); err != nil {
				return err
			}
			revKey = &r
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("unable to get revision key: %w", err)
	}
	if revKey == nil {
		return nil, fmt.Errorf("no effective key found")
	}

	// Unwrap the DEK w/ the KeyManager.
	unwrapped, err := rdb.decrypt(ctx, revKey.WrappedCipher)
	if err != nil {
		return nil, fmt.Errorf("unable to unwrap key: %w", err)
	}
	revKey.DEK = unwrapped

	return revKey, nil
}

func (rdb *RevisionDB) decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	return rdb.config.KeyManager.Decrypt(ctx, rdb.config.WrapperKeyID, ciphertext, rdb.config.WrapperAAD)
}

// CreateRevisionKey generates a new AES key and wraps it
func (rdb *RevisionDB) CreateRevisionKey(ctx context.Context) (*RevisionKey, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("unable to generate AES key: %w", err)
	}
	aad := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, aad); err != nil {
		return nil, fmt.Errorf("unable to generate random data: %w", err)
	}

	// Wrap the key using the configured KMS.
	wrapped, err := rdb.config.KeyManager.Encrypt(ctx, rdb.config.WrapperKeyID, key, rdb.config.WrapperAAD)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap key: %w", err)
	}

	// Start building the RevisionKey
	revKey := RevisionKey{
		WrappedCipher: wrapped,
		AAD:           aad,
		CreatedAt:     time.Now().UTC(),
		Allowed:       true,
		DEK:           key,
	}

	if err := rdb.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				RevisionKeys
				(aad, wrapped_cipher, created_at, allowed)
			VALUES
				($1, $2, $3, $4)
			RETURNING kid`,
			revKey.AAD, wrapped, revKey.CreatedAt, true)
		if err := row.Scan(&revKey.KeyID); err != nil {
			return fmt.Errorf("fetching kid: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("unable to persist revision key: %w", err)
	}

	return &revKey, nil
}
