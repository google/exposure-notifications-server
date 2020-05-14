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

package database

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/model"
	pgx "github.com/jackc/pgx/v4"
)

// AddFederationOutAuthorization adds or updates a FederationOutAuthorization record.
func (db *DB) AddFederationOutAuthorization(ctx context.Context, auth *model.FederationOutAuthorization) error {
	return db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		q := `
			INSERT INTO
				FederationOutAuthorization
				(oidc_issuer, oidc_subject, oidc_audience, note, include_regions, exclude_regions)
			VALUES
				($1, $2, $3, $4, $5, $6)
			ON CONFLICT ON CONSTRAINT
				federation_authorization_pk
			DO UPDATE
				SET oidc_audience = $3, note = $4, include_regions = $5, exclude_regions = $6
		`
		_, err := tx.Exec(ctx, q, auth.Issuer, auth.Subject, auth.Audience, auth.Note, auth.IncludeRegions, auth.ExcludeRegions)
		if err != nil {
			return fmt.Errorf("upserting federation authorization: %w", err)
		}
		return nil
	})
}

// GetFederationOutAuthorization returns a FederationOutAuthorization record, or ErrNotFound if not found.
func (db *DB) GetFederationOutAuthorization(ctx context.Context, issuer, subject string) (*model.FederationOutAuthorization, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			oidc_issuer, oidc_subject, oidc_audience, note, include_regions, exclude_regions
		FROM
			FederationOutAuthorization
		WHERE
			oidc_issuer = $1
		AND
			oidc_subject = $2
		LIMIT 1
		`, issuer, subject)
	auth := model.FederationOutAuthorization{}
	if err := row.Scan(&auth.Issuer, &auth.Subject, &auth.Audience, &auth.Note, &auth.IncludeRegions, &auth.ExcludeRegions); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning results: %w", err)
	}
	return &auth, nil
}
