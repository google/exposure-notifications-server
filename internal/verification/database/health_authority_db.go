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

// Package database is a database interface to health authorities.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/database"

	pgx "github.com/jackc/pgx/v4"
)

var ErrHealthAuthorityNotFound = errors.New("health authority not found")

// HealthAuthorityDB allows for opreations against authorized health authorities
// for diagnosis signature verification.
type HealthAuthorityDB struct {
	db *database.DB
}

// New creates a HealthAuthorityDB attached to a specific database driver.
func New(db *database.DB) *HealthAuthorityDB {
	return &HealthAuthorityDB{db}
}

// AddHealthAuthority inserts a new HealthAuthority record into the database.
func (db *HealthAuthorityDB) AddHealthAuthority(ctx context.Context, ha *model.HealthAuthority) error {
	if ha == nil {
		return errors.New("provided HealthAuthority cannot be nil")
	}
	if len(ha.Keys) != 0 {
		return fmt.Errorf("unable to insert health authority with keys, attach keys later")
	}
	if err := ha.Validate(); err != nil {
		return err
	}

	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				HealthAuthority
				(iss, aud, name, jwks_uri, enable_stats)
			VALUES
				($1, $2, $3, $4, $5)
			RETURNING id
			`, ha.Issuer, ha.Audience, ha.Name, ha.JwksURI, ha.EnableStatsAPI)
		if err := row.Scan(&ha.ID); err != nil {
			return fmt.Errorf("inserting healthauthority: %w", err)
		}
		return nil
	})
}

// UpdateHealthAuthority updates the database record for the specified health authority.
func (db *HealthAuthorityDB) UpdateHealthAuthority(ctx context.Context, ha *model.HealthAuthority) error {
	if ha == nil {
		return errors.New("provided HealthAuthority cannot be nil")
	}
	if err := ha.Validate(); err != nil {
		return err
	}

	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE HealthAuthority
			SET
				iss = $1, aud = $2, name = $3, jwks_uri = $4, enable_stats = $5
			WHERE
				id = $6
			`, ha.Issuer, ha.Audience, ha.Name, ha.JwksURI, ha.EnableStatsAPI, ha.ID)
		if err != nil {
			return fmt.Errorf("updating health authority: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updates")
		}
		return nil
	})
}

func (db *HealthAuthorityDB) GetHealthAuthorityByID(ctx context.Context, id int64) (*model.HealthAuthority, error) {
	var ha *model.HealthAuthority

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, iss, aud, name, jwks_uri, enable_stats
			FROM
				HealthAuthority
			WHERE
				id = $1
		`, id)

		var err error
		ha, err = scanOneHealthAuthority(row)
		if err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("get health authority by id: %w", err)
	}

	haks, err := db.GetHealthAuthorityKeys(ctx, ha)
	if err != nil {
		return nil, err
	}
	ha.Keys = haks

	return ha, nil
}

// GetHealthAuthority retrieves a HealthAuthority record by the issuer name.
func (db *HealthAuthorityDB) GetHealthAuthority(ctx context.Context, issuer string) (*model.HealthAuthority, error) {
	var ha *model.HealthAuthority

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, iss, aud, name, jwks_uri, enable_stats
			FROM
				HealthAuthority
			WHERE
				iss = $1
		`, issuer)

		var err error
		ha, err = scanOneHealthAuthority(row)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHealthAuthorityNotFound
		}
		return nil, fmt.Errorf("get health authority: %w", err)
	}

	haks, err := db.GetHealthAuthorityKeys(ctx, ha)
	if err != nil {
		return nil, err
	}
	ha.Keys = haks

	return ha, nil
}

// ListAllHealthAuthoritiesWithoutKeys retrieves all known health authorities in the system.
// Support function for admin console.
func (db *HealthAuthorityDB) ListAllHealthAuthoritiesWithoutKeys(ctx context.Context) ([]*model.HealthAuthority, error) {
	var has []*model.HealthAuthority

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, iss, aud, name, jwks_uri, enable_stats
			FROM
				HealthAuthority
			ORDER BY iss ASC
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			ha, err := scanOneHealthAuthority(rows)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			has = append(has, ha)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list health authorities: %w", err)
	}

	return has, nil
}

func scanOneHealthAuthority(row pgx.Row) (*model.HealthAuthority, error) {
	var ha model.HealthAuthority
	if err := row.Scan(&ha.ID, &ha.Issuer, &ha.Audience, &ha.Name, &ha.JwksURI, &ha.EnableStatsAPI); err != nil {
		return nil, err
	}
	return &ha, nil
}

func (db *HealthAuthorityDB) AddHealthAuthorityKey(ctx context.Context, ha *model.HealthAuthority, hak *model.HealthAuthorityKey) error {
	if ha == nil {
		return errors.New("provided HealthAuthority cannot be nil")
	}
	if hak == nil {
		return errors.New("provided HealthAuthorityKey cannot be nil")
	}

	if err := hak.Validate(); err != nil {
		return err
	}
	if ha.ID == 0 {
		return errors.New("invalid health authority ID, must be non zero")
	}

	hak.AuthorityID = ha.ID
	thru := database.NullableTime(hak.Thru)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO
				HealthAuthorityKey
				(health_authority_id, version, from_timestamp, thru_timestamp, public_key)
			VALUES
				($1, $2, $3, $4, $5)
			`, hak.AuthorityID, hak.Version, hak.From, thru, hak.PublicKeyPEM)
		if err != nil {
			return fmt.Errorf("inserting healthauthoritykey: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows inserted")
		}
		return nil
	})
}

func (db *HealthAuthorityDB) PurgeHealthAuthorityKeys(ctx context.Context, ha *model.HealthAuthority, purgeBefore time.Time) (int64, error) {
	purgeCount := int64(0)
	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM HealthAuthorityKey
			WHERE
				health_authority_id = $1 AND (thru_timestamp IS NOT NULL and thru_timestamp < $2)
			`, ha.ID, purgeBefore)
		if err != nil {
			return fmt.Errorf("updating health authority key: %w", err)
		}
		purgeCount = result.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return purgeCount, nil
}

func (db *HealthAuthorityDB) UpdateHealthAuthorityKey(ctx context.Context, hak *model.HealthAuthorityKey) error {
	if _, err := hak.PublicKey(); err != nil {
		return err
	}

	thru := database.NullableTime(hak.Thru)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE HealthAuthorityKey
			SET
				from_timestamp = $1, thru_timestamp = $2, public_key = $3
			WHERE
				health_authority_id = $4 AND version = $5
			`, hak.From, thru, hak.PublicKeyPEM, hak.AuthorityID, hak.Version)
		if err != nil {
			return fmt.Errorf("updating health authority key: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

func (db *HealthAuthorityDB) GetHealthAuthorityKeys(ctx context.Context, ha *model.HealthAuthority) ([]*model.HealthAuthorityKey, error) {
	var keys []*model.HealthAuthorityKey

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				health_authority_id, version, from_timestamp, thru_timestamp, public_key
			FROM
				HealthAuthorityKey
			WHERE
				health_authority_id = $1
			ORDER BY
				version
		`, ha.ID)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var key model.HealthAuthorityKey
			var thru *time.Time
			if err := rows.Scan(&key.AuthorityID, &key.Version, &key.From, &thru, &key.PublicKeyPEM); err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			if thru != nil {
				key.Thru = *thru
			}
			keys = append(keys, &key)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("get health authority keys: %w", err)
	}

	return keys, nil
}
