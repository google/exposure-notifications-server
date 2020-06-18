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
	"github.com/google/exposure-notifications-server/pkg/secrets"
	pgx "github.com/jackc/pgx/v4"
)

type AuthorizedAppDB struct {
	db *database.DB
}

func New(db *database.DB) *AuthorizedAppDB {
	return &AuthorizedAppDB{
		db: db,
	}
}

func (aa *AuthorizedAppDB) InsertAuthorizedApp(ctx context.Context, m *model.AuthorizedApp) error {
	if errors := m.Validate(); len(errors) > 0 {
		return fmt.Errorf("AuthorizedApp invalid: %v", strings.Join(errors, ", "))
	}

	return aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO
				AuthorizedApp
				(app_package_name, allowed_regions,
				allowed_health_authority_ids, bypass_health_authority_verification)
			VALUES
				(LOWER($1), $2, $3, $4)
		`, m.AppPackageName, m.AllAllowedRegions(),
			m.AllAllowedHealthAuthorityIDs(), m.BypassHealthAuthorityVerification)

		if err != nil {
			return fmt.Errorf("inserting authorizedapp: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows inserted")
		}
		return nil
	})
}

func (aa *AuthorizedAppDB) UpdateAuthorizedApp(ctx context.Context, priorKey string, m *model.AuthorizedApp) error {
	return aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE AuthorizedApp
			SET
				app_package_name = LOWER($1), allowed_regions = $2,
				allowed_health_authority_ids = $3, bypass_health_authority_verification = $4
			WHERE
				LOWER(app_package_name) = LOWER($5)
			`, m.AppPackageName, m.AllAllowedRegions(),
			m.AllAllowedHealthAuthorityIDs(), m.BypassHealthAuthorityVerification,
			priorKey)
		if err != nil {
			return fmt.Errorf("updating authorizedapp: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

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

func (aa *AuthorizedAppDB) ListAuthorizedApps(ctx context.Context) ([]*model.AuthorizedApp, error) {
	conn, err := aa.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	query := `
		SELECT
			LOWER(app_package_name), allowed_regions,
			allowed_health_authority_ids, bypass_health_authority_verification
		FROM
			AuthorizedApp
		ORDER BY LOWER(app_package_name) ASC`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.AuthorizedApp
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating rows: %w", err)
		}

		app, err := scanOneAuthorizedApp(ctx, rows)
		if err != nil {
			return nil, fmt.Errorf("error reading authorized apps: %w", err)
		}
		result = append(result, app)
	}
	return result, nil
}

// GetAuthorizedApp loads a single AuthorizedApp for the given name. If no row
// exists, this returns nil.
func (aa *AuthorizedAppDB) GetAuthorizedApp(ctx context.Context, sm secrets.SecretManager, name string) (*model.AuthorizedApp, error) {
	conn, err := aa.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	query := `
		SELECT
			LOWER(app_package_name), allowed_regions,
			allowed_health_authority_ids, bypass_health_authority_verification
		FROM
			AuthorizedApp
		WHERE LOWER(app_package_name) = LOWER($1)`

	row := conn.QueryRow(ctx, query, name)

	return scanOneAuthorizedApp(ctx, row)
}

func scanOneAuthorizedApp(ctx context.Context, row pgx.Row) (*model.AuthorizedApp, error) {
	config := model.NewAuthorizedApp()
	var allowedRegions []string
	var allowedHealthAuthorityIDs []int64
	if err := row.Scan(
		&config.AppPackageName, &allowedRegions,
		&allowedHealthAuthorityIDs, &config.BypassHealthAuthorityVerification,
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
