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

// Package database is a database interface to authorized apps.
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/database"
	pgx "github.com/jackc/pgx/v4"
)

// AuthorizedAppDB is a handle to database operations for authorized apps
// (referred to as healthAuthorityID in v1 publish API).
type AuthorizedAppDB struct {
	db *database.DB
}

// New creates a new authorizedAppDB that wraps a raw database handle.
func New(db *database.DB) *AuthorizedAppDB {
	return &AuthorizedAppDB{
		db: db,
	}
}

// InsertAuthorizedApp inserts an authorized app into the database, caling the validate method first
// and returning any errors.
func (aa *AuthorizedAppDB) InsertAuthorizedApp(ctx context.Context, m *model.AuthorizedApp) error {
	if errors := m.Validate(); len(errors) > 0 {
		return fmt.Errorf("AuthorizedApp invalid: %v", strings.Join(errors, ", "))
	}

	return aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO
				AuthorizedApp
				(app_package_name, allowed_regions,
				allowed_health_authority_ids, bypass_health_authority_verification, bypass_revision_token)
			VALUES
				(LOWER($1), $2, $3, $4, $5)
		`, m.AppPackageName, m.AllAllowedRegions(),
			m.AllAllowedHealthAuthorityIDs(), m.BypassHealthAuthorityVerification,
			m.BypassRevisionToken)

		if err != nil {
			return fmt.Errorf("inserting authorizedapp: %w", err)
		}
		return nil
	})
}

// UpdateAuthorizedApp updates the properties of an authorized app, including possibly renaming it.
func (aa *AuthorizedAppDB) UpdateAuthorizedApp(ctx context.Context, priorKey string, m *model.AuthorizedApp) error {
	return aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE AuthorizedApp
			SET
				app_package_name = LOWER($1), allowed_regions = $2,
				allowed_health_authority_ids = $3, bypass_health_authority_verification = $4,
				bypass_revision_token = $5
			WHERE
				LOWER(app_package_name) = LOWER($6)
			`, m.AppPackageName, m.AllAllowedRegions(),
			m.AllAllowedHealthAuthorityIDs(), m.BypassHealthAuthorityVerification,
			m.BypassRevisionToken, priorKey)
		if err != nil {
			return fmt.Errorf("updating authorizedapp: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

// DeleteAuthorizedApp removes an authorized app from the database.
func (aa *AuthorizedAppDB) DeleteAuthorizedApp(ctx context.Context, name string) error {
	var count int64
	err := aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM
				AuthorizedApp
			WHERE
				LOWER(app_package_name) = LOWER($1)
			`, name)
		if err != nil {
			return fmt.Errorf("deleting authorized app: %w", err)
		}
		count = result.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no rows were deleted")
	}
	return nil
}

// ListAuthorizedApps reads all authorized app, returned in alphabetical order by
// healthAuthorityID (app_package_name).
func (aa *AuthorizedAppDB) ListAuthorizedApps(ctx context.Context) ([]*model.AuthorizedApp, error) {
	var apps []*model.AuthorizedApp

	if err := aa.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				LOWER(app_package_name), allowed_regions,
				allowed_health_authority_ids, bypass_health_authority_verification, bypass_revision_token
			FROM
				AuthorizedApp
			ORDER BY LOWER(app_package_name) ASC
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			app, err := scanOneAuthorizedApp(rows)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			apps = append(apps, app)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("list authorized apps: %w", err)
	}

	return apps, nil
}

// GetAuthorizedApp loads a single AuthorizedApp for the given name. If no row
// exists, this returns nil.
func (aa *AuthorizedAppDB) GetAuthorizedApp(ctx context.Context, name string) (*model.AuthorizedApp, error) {
	var app *model.AuthorizedApp

	if err := aa.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				LOWER(app_package_name), allowed_regions,
				allowed_health_authority_ids, bypass_health_authority_verification, bypass_revision_token
			FROM
				AuthorizedApp
			WHERE LOWER(app_package_name) = LOWER($1)
		`, name)

		var err error
		app, err = scanOneAuthorizedApp(row)
		if err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("get authorized app: %w", err)
	}

	return app, nil
}

func scanOneAuthorizedApp(row pgx.Row) (*model.AuthorizedApp, error) {
	config := model.NewAuthorizedApp()
	var allowedRegions []string
	var allowedHealthAuthorityIDs []int64
	if err := row.Scan(
		&config.AppPackageName, &allowedRegions,
		&allowedHealthAuthorityIDs, &config.BypassHealthAuthorityVerification,
		&config.BypassRevisionToken,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// build the regions map
	for _, r := range allowedRegions {
		config.AllowedRegions[r] = struct{}{}
	}
	// and the health authorities map
	for _, haID := range allowedHealthAuthorityIDs {
		config.AllowedHealthAuthorityIDs[haID] = struct{}{}
	}

	return config, nil
}
