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
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest"
	"github.com/sethvargo/go-retry"

	// imported to register the postgres migration driver
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// imported to register the "file" source migration driver
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// imported to register the "postgres" database driver for migrate
)

// NewTestDatabaseWithConfig creates a new database suitable for use in testing.
// This should not be used outside of testing, but it is exposed in the main
// package so it can be shared with other packages.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestDatabaseWithConfig(tb testing.TB) (*DB, *Config) {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping database tests (short!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_DATABASE_TESTS")); skip {
		tb.Skipf("🚧 Skipping database tests (SKIP_DATABASE_TESTS is set)!")
	}

	// Context.
	ctx := context.Background()

	// Create the pool (docker instance).
	pool, err := dockertest.NewPool("")
	if err != nil {
		tb.Fatalf("failed to create Docker pool: %s", err)
	}

	// Determine the container image to use.
	repo, tag := postgresRepo(tb)

	// Start the container.
	dbname, username, password := "en-server", "my-username", "abcd1234"
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: repo,
		Tag:        tag,
		Env: []string{
			"LANG=C",
			"POSTGRES_DB=" + dbname,
			"POSTGRES_USER=" + username,
			"POSTGRES_PASSWORD=" + password,
		},
	})
	if err != nil {
		tb.Fatalf("failed to start postgres container: %s", err)
	}

	// Force the database container to stop.
	if err := container.Expire(120); err != nil {
		tb.Fatalf("failed to force-stop container: %v", err)
	}

	// Ensure container is cleaned up.
	tb.Cleanup(func() {
		if err := pool.Purge(container); err != nil {
			tb.Fatalf("failed to cleanup postgres container: %s", err)
		}
	})

	// Get the host. On Mac, Docker runs in a VM.
	host := container.Container.NetworkSettings.IPAddress
	if runtime.GOOS == "darwin" {
		host = net.JoinHostPort(container.GetBoundIP("5432/tcp"), container.GetPort("5432/tcp"))
	}

	// Build the connection URL.
	connURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(username, password),
		Host:   host,
		Path:   dbname,
	}
	q := connURL.Query()
	q.Add("sslmode", "disable")
	connURL.RawQuery = q.Encode()

	// Wait for the container to start.
	b, err := retry.NewConstant(1 * time.Second)
	if err != nil {
		tb.Fatalf("failed to configure backoff: %v", err)
	}
	b = retry.WithMaxRetries(30, b)

	// Establish a connection to the database. Use a Fibonacci backoff instead of
	// exponential so wait times scale appropriately.
	var dbpool *pgxpool.Pool
	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		var err error
		dbpool, err = pgxpool.Connect(ctx, connURL.String())
		if err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		tb.Fatalf("failed to start postgres: %s", err)
	}

	// Run the migrations.
	if err := dbMigrate(connURL.String()); err != nil {
		tb.Fatalf("failed to migrate database: %s", err)
	}

	// Create the db instance.
	db := &DB{Pool: dbpool}

	// Close db when done.
	tb.Cleanup(func() {
		db.Close(context.Background())
	})

	return db, &Config{
		Name:     dbname,
		User:     username,
		Host:     container.GetBoundIP("5432/tcp"),
		Port:     container.GetPort("5432/tcp"),
		SSLMode:  "disable",
		Password: password,
	}
}

func NewTestDatabase(tb testing.TB) *DB {
	tb.Helper()

	db, _ := NewTestDatabaseWithConfig(tb)
	return db
}

// dbMigrate runs the migrations. u is the connection URL string (e.g.
// postgres://...).
func dbMigrate(u string) error {
	// Run the migrations
	migrationsDir := fmt.Sprintf("file://%s", dbMigrationsDir())
	m, err := migrate.New(migrationsDir, u)
	if err != nil {
		return fmt.Errorf("failed create migrate: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed run migrate: %w", err)
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return fmt.Errorf("migrate source error: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("migrate database error: %w", dbErr)
	}
	return nil
}

// dbMigrationsDir returns the path on disk to the migrations. It uses
// runtime.Caller() to get the path to the caller, since this package is
// imported by multiple others at different levels.
func dbMigrationsDir() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(filename), "../../migrations")
}

func postgresRepo(tb testing.TB) (string, string) {
	postgresImageRef := os.Getenv("CI_POSTGRES_IMAGE")
	if postgresImageRef == "" {
		postgresImageRef = "postgres:13-alpine"
	}

	parts := strings.SplitN(postgresImageRef, ":", 2)
	if len(parts) != 2 {
		tb.Fatalf("invalid postgres ref %v", postgresImageRef)
	}
	return parts[0], parts[1]
}
