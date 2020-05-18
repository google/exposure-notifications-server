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
	"time"

	"github.com/google/exposure-notifications-server/internal/ios"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/secrets"
	pgx "github.com/jackc/pgx/v4"
)

// GetAuthorizedApp loads a single AuthorizedApp for the given name. If no row
// exists, this returns nil.
func (db *DB) GetAuthorizedApp(ctx context.Context, sm secrets.SecretManager, name string) (*model.AuthorizedApp, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	query := `
		SELECT
			app_package_name, platform, apk_digest, cts_profile_match, basic_integrity,
			allowed_past_seconds, allowed_future_seconds, allowed_regions, all_regions,
			ios_devicecheck_team_id_secret, ios_devicecheck_key_id_secret, ios_devicecheck_private_key_secret
		FROM
			AuthorizedApp
		WHERE app_package_name = $1`

	row := conn.QueryRow(ctx, query, name)

	config := model.NewAuthorizedApp()
	var allowedRegions []string
	var allowedPastSeconds, allowedFutureSeconds *int
	var deviceCheckTeamIDSecret, deviceCheckKeyIDSecret, deviceCheckPrivateKeySecret sql.NullString
	if err := row.Scan(
		&config.AppPackageName, &config.Platform, &config.ApkDigestSHA256, &config.CTSProfileMatch, &config.BasicIntegrity,
		&allowedPastSeconds, &allowedFutureSeconds, &allowedRegions, &config.AllowAllRegions,
		&deviceCheckTeamIDSecret, &deviceCheckKeyIDSecret, &deviceCheckPrivateKeySecret,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert time in seconds from DB into time.Duration
	if allowedPastSeconds != nil {
		d := time.Duration(*allowedPastSeconds) * time.Second
		config.AllowedPastTime = d
	}
	if allowedFutureSeconds != nil {
		d := time.Duration(*allowedFutureSeconds) * time.Second
		config.AllowedFutureTime = d
	}

	// build the regions map
	for _, r := range allowedRegions {
		config.AllowedRegions[r] = struct{}{}
	}

	// Resolve secrets to their plaintext values
	if v := deviceCheckTeamIDSecret; v.Valid && v.String != "" {
		plaintext, err := sm.GetSecretValue(ctx, v.String)
		if err != nil {
			return nil, fmt.Errorf("ios_devicecheck_team_id_secret at %s (%s): %w",
				config.AppPackageName, config.Platform, err)
		}
		config.DeviceCheckTeamID = plaintext
	}

	if v := deviceCheckKeyIDSecret; v.Valid && v.String != "" {
		plaintext, err := sm.GetSecretValue(ctx, v.String)
		if err != nil {
			return nil, fmt.Errorf("ios_devicecheck_key_id_secret at %s (%s): %w",
				config.AppPackageName, config.Platform, err)
		}
		config.DeviceCheckKeyID = plaintext
	}

	if v := deviceCheckPrivateKeySecret; v.Valid && v.String != "" {
		plaintext, err := sm.GetSecretValue(ctx, v.String)
		if err != nil {
			return nil, fmt.Errorf("ios_devicecheck_private_key_secret at %s (%s): %w",
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
