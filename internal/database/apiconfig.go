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

	"github.com/google/exposure-notifications-server/internal/model/apiconfig"
)

// ReadAPIConfigs loads all APIConfig values from the database.
func (db *DB) ReadAPIConfigs(ctx context.Context) ([]*apiconfig.APIConfig, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	query := `
	    SELECT
	    	app_package_name, platform, apk_digest, enforce_apk_digest, cts_profile_match, basic_integrity,
        allowed_past_seconds, allowed_future_seconds, allowed_regions, all_regions, bypass_safetynet
	    FROM
	    	APIConfig`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// In most instances, we expect a single config entry.
	var result []*apiconfig.APIConfig
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating rows: %v", err)
		}

		var regions []string
		config := apiconfig.New()
		var apkDigest sql.NullString
		var allowedPastSeconds, allowedFutureSeconds *int
		if err := rows.Scan(&config.AppPackageName, &config.Platform, &apkDigest,
			&config.EnforceApkDigest, &config.CTSProfileMatch, &config.BasicIntegrity,
			&allowedPastSeconds, &allowedFutureSeconds, &regions,
			&config.AllowAllRegions, &config.BypassSafetynet); err != nil {
			return nil, err
		}
		if apkDigest.Valid {
			config.ApkDigestSHA256 = apkDigest.String
		}

		// Convert time in seconds from DB into time.Duration
		if allowedPastSeconds != nil {
			d := time.Duration(*allowedPastSeconds) * time.Second
			config.AllowedPastTime = &d
		}
		if allowedFutureSeconds != nil {
			d := time.Duration(*allowedFutureSeconds) * time.Second
			config.AllowedFutureTime = &d
		}

		// build the regions map
		for _, r := range regions {
			config.AllowedRegions[r] = true
		}

		result = append(result, config)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
