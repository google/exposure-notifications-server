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

package integration

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/monolith"
)

func SetEnvAndRunServer(tb testing.TB, ctx context.Context, dbConfig *database.Config) {

	// Set all of these to not need to connect to external resources.
	os.Setenv("BLOBSTORE", "FILESYSTEM")
	os.Setenv("KEY_MANAGER", "NOOP")
	os.Setenv("SECRET_MANAGER", "NOOP")

	// Update database environment variables.
	os.Setenv("DB_NAME", dbConfig.Name)
	os.Setenv("DB_USER", dbConfig.User)
	os.Setenv("DB_HOST", dbConfig.Host)
	os.Setenv("DB_PORT", dbConfig.Port)
	os.Setenv("DB_SSLMODE", dbConfig.SSLMode)
	os.Setenv("DB_CONNECT_TIMEOUT", strconv.Itoa(dbConfig.ConnectionTimeout))
	os.Setenv("DB_PASSWORD", dbConfig.Password)
	os.Setenv("DB_SSLCERT", dbConfig.SSLCertPath)
	os.Setenv("DB_SSLKEY", dbConfig.SSLKeyPath)
	os.Setenv("DB_SSLROOTCERT", dbConfig.SSLRootCertPath)
	os.Setenv("DB_POOL_MIN_CONNS", dbConfig.PoolMinConnections)
	os.Setenv("DB_POOL_MAX_CONNS", dbConfig.PoolMaxConnections)
	os.Setenv("DB_POOL_MAX_CONN_LIFETIME", dbConfig.PoolMaxConnLife.String())
	os.Setenv("DB_POOL_MAX_CONN_IDLE_TIME", dbConfig.PoolMaxConnIdle.String())
	os.Setenv("DB_POOL_HEALTH_CHECK_PERIOD", dbConfig.PoolHealthCheck.String())

	if _, err := monolith.RunServer(ctx); !errors.Is(err, http.ErrServerClosed) {
		tb.Fatalf("Failed to start Monolith.  Error: %+v", err)
	}
}

func StartSystemUnderTest(tb testing.TB, ctx context.Context) (*database.DB, *monolith.MonoConfig) {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping integration tests (short!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_INTEGRATION_TESTS")); skip {
		tb.Skipf("🚧 Skipping integration tests (SKIP_INTEGRATION_TESTS is set)!")
	}

	db, dbconfig := database.NewTestDatabaseWithConfig(tb)

	tb.Cleanup(func() {
		http.Get("http://localhost:8080/shutdown")
	})

	go SetEnvAndRunServer(tb, ctx, dbconfig)

	return db, nil

}
