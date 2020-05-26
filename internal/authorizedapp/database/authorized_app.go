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
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/ios"
	"github.com/google/exposure-notifications-server/internal/secrets"
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
			  (app_package_name, platform, allowed_regions,
			   safetynet_disabled, safetynet_apk_digest, safetynet_cts_profile_match, safetynet_basic_integrity, safetynet_past_seconds, safetynet_future_seconds,
			   devicecheck_disabled, devicecheck_team_id, devicecheck_key_id, devicecheck_private_key_secret)
			VALUES
			  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, m.AppPackageName, m.Platform, m.AllAllowedRegions(),
			m.SafetyNetDisabled, m.SafetyNetApkDigestSHA256, m.SafetyNetCTSProfileMatch, m.SafetyNetBasicIntegrity, int64(m.SafetyNetPastTime.Seconds()), int64(m.SafetyNetFutureTime.Seconds()),
			m.DeviceCheckDisabled, m.DeviceCheckTeamID, m.DeviceCheckKeyID, m.DeviceCheckPrivateKeySecret)

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
				app_package_name = $1, platform = $2, allowed_regions = $3,
				safetynet_disabled = $4, safetynet_apk_digest = $5, safetynet_cts_profile_match = $6, safetynet_basic_integrity = $7, safetynet_past_seconds = $8, safetynet_future_seconds = $9,
				devicecheck_disabled = $10, devicecheck_team_id = $11, devicecheck_key_id = $12, devicecheck_private_key_secret = $13
			WHERE
				app_package_name = $14
			`, m.AppPackageName, m.Platform, m.AllAllowedRegions(),
			m.SafetyNetDisabled, m.SafetyNetApkDigestSHA256, m.SafetyNetCTSProfileMatch, m.SafetyNetBasicIntegrity, int64(m.SafetyNetPastTime.Seconds()), int64(m.SafetyNetFutureTime.Seconds()),
			m.DeviceCheckDisabled, m.DeviceCheckTeamID, m.DeviceCheckKeyID, m.DeviceCheckPrivateKeySecret, priorKey)
		if err != nil {
			return fmt.Errorf("updating authorizedapp: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

func (aa *AuthorizedAppDB) DeleteAuthorizedApp(ctx context.Context, appPackageName string) error {
	var count int64
	err := aa.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM
				AuthorizedApp
			WHERE
				app_package_name = $1
			`, appPackageName)
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

func (aa *AuthorizedAppDB) GetAllAuthorizedApps(ctx context.Context, sm secrets.SecretManager) ([]*model.AuthorizedApp, error) {
	conn, err := aa.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	query := `
    SELECT
      app_package_name, platform, allowed_regions,
      safetynet_disabled, safetynet_apk_digest, safetynet_cts_profile_match, safetynet_basic_integrity, safetynet_past_seconds, safetynet_future_seconds,
      devicecheck_disabled, devicecheck_team_id, devicecheck_key_id, devicecheck_private_key_secret
    FROM
      AuthorizedApp
    ORDER BY app_package_name ASC`

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

		app, err := scanOneAuthorizedApp(ctx, rows, sm)
		if err != nil {
			return nil, fmt.Errorf("error reading authorized apps: %w", err)
		}
		result = append(result, app)
	}
	return result, nil
}

// GetAuthorizedApp loads a single AuthorizedApp for the given name. If no row
// exists, this returns nil.
func (db *AuthorizedAppDB) GetAuthorizedApp(ctx context.Context, sm secrets.SecretManager, name string) (*model.AuthorizedApp, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	query := `
		SELECT
			app_package_name, platform, allowed_regions,
			safetynet_disabled, safetynet_apk_digest, safetynet_cts_profile_match, safetynet_basic_integrity, safetynet_past_seconds, safetynet_future_seconds,
			devicecheck_disabled, devicecheck_team_id, devicecheck_key_id, devicecheck_private_key_secret
		FROM
			AuthorizedApp
		WHERE app_package_name = $1`

	row := conn.QueryRow(ctx, query, name)

	return scanOneAuthorizedApp(ctx, row, sm)
}

func scanOneAuthorizedApp(ctx context.Context, row pgx.Row, sm secrets.SecretManager) (*model.AuthorizedApp, error) {
	config := model.NewAuthorizedApp()
	var allowedRegions []string
	var safetyNetPastSeconds, safetyNetFutureSeconds *int
	var deviceCheckTeamID, deviceCheckKeyID, deviceCheckPrivateKeySecret sql.NullString
	if err := row.Scan(
		&config.AppPackageName, &config.Platform, &allowedRegions,
		&config.SafetyNetDisabled, &config.SafetyNetApkDigestSHA256, &config.SafetyNetCTSProfileMatch, &config.SafetyNetBasicIntegrity, &safetyNetPastSeconds, &safetyNetFutureSeconds,
		&config.DeviceCheckDisabled, &deviceCheckTeamID, &deviceCheckKeyID, &deviceCheckPrivateKeySecret,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert time in seconds from DB into time.Duration
	if safetyNetPastSeconds != nil {
		d := time.Duration(*safetyNetPastSeconds) * time.Second
		config.SafetyNetPastTime = d
	}
	if safetyNetFutureSeconds != nil {
		d := time.Duration(*safetyNetFutureSeconds) * time.Second
		config.SafetyNetFutureTime = d
	}

	// build the regions map
	for _, r := range allowedRegions {
		config.AllowedRegions[r] = struct{}{}
	}

	// Handle nulls
	if v := deviceCheckTeamID; v.Valid && v.String != "" {
		config.DeviceCheckTeamID = v.String
	}

	if v := deviceCheckKeyID; v.Valid && v.String != "" {
		config.DeviceCheckKeyID = v.String
	}

	// Resolve secrets to their plaintext values
	if v := deviceCheckPrivateKeySecret; v.Valid && v.String != "" {
		config.DeviceCheckPrivateKeySecret = v.String
		plaintext, err := sm.GetSecretValue(ctx, v.String)
		if err != nil {
			return nil, fmt.Errorf("devicecheck_private_key_secret at %s (%s): %w",
				config.AppPackageName, config.Platform, err)
		}

		key, err := ios.ParsePrivateKey(plaintext)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key at %s (%s): %w",
				config.AppPackageName, config.Platform, err)
		}
		config.DeviceCheckPrivateKey = key
	}

	return config, nil
}
