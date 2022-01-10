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

package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sethvargo/go-retry"

	// imported to register the postgres migration driver
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// imported to register the "file" source migration driver
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// imported to register the "postgres" database driver for migrate
)

const (
	// databaseName is the name of the template database to clone.
	databaseName = "test-db-template"

	// databaseUser and databasePassword are the username and password for
	// connecting to the database. These values are only used for testing.
	databaseUser     = "test-user"
	databasePassword = "testing123"

	// defaultPostgresImageRef is the default database container to use if none is
	// specified.
	defaultPostgresImageRef = "postgres:13-alpine"
)

// ApproxTime is a compare helper for clock skew.
var ApproxTime = cmp.Options{cmpopts.EquateApproxTime(1 * time.Second)}

// TestInstance is a wrapper around the Docker-based database instance.
type TestInstance struct {
	pool      *dockertest.Pool
	container *dockertest.Resource
	url       *url.URL

	conn     *pgx.Conn
	connLock sync.Mutex

	skipReason string
}

// MustTestInstance is NewTestInstance, except it prints errors to stderr and
// calls os.Exit when finished. Callers can call Close or MustClose().
func MustTestInstance() *TestInstance {
	testDatabaseInstance, err := NewTestInstance()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return testDatabaseInstance
}

// NewTestInstance creates a new Docker-based database instance. It also creates
// an initial database, runs the migrations, and sets that database as a
// template to be cloned by future tests.
//
// This should not be used outside of testing, but it is exposed in the package
// so it can be shared with other packages. It should be called and instantiated
// in TestMain.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestInstance() (*TestInstance, error) {
	// Querying for -short requires flags to be parsed.
	if !flag.Parsed() {
		flag.Parse()
	}

	// Do not create an instance in -short mode.
	if testing.Short() {
		return &TestInstance{
			skipReason: "🚧 Skipping database tests (-short flag provided)!",
		}, nil
	}

	// Do not create an instance if database tests are explicitly skipped.
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_DATABASE_TESTS")); skip {
		return &TestInstance{
			skipReason: "🚧 Skipping database tests (SKIP_DATABASE_TESTS is set)!",
		}, nil
	}

	ctx := context.Background()

	// Create the pool.
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("failed to create database docker pool: %w", err)
	}

	// Determine the container image to use.
	repository, tag, err := postgresRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to determine database repository: %w", err)
	}

	// Start the actual container.
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: repository,
		Tag:        tag,
		Env: []string{
			"LANG=C",
			"POSTGRES_DB=" + databaseName,
			"POSTGRES_USER=" + databaseUser,
			"POSTGRES_PASSWORD=" + databasePassword,
		},
	}, func(c *docker.HostConfig) {
		c.AutoRemove = true
		c.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start database container: %w", err)
	}

	// Stop the container after its been running for too long. No since test suite
	// should take super long.
	if err := container.Expire(120); err != nil {
		return nil, fmt.Errorf("failed to expire database container: %w", err)
	}

	// Build the connection URL.
	connectionURL := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(databaseUser, databasePassword),
		Host:     container.GetHostPort("5432/tcp"),
		Path:     databaseName,
		RawQuery: "sslmode=disable",
	}

	// Create retryable.
	b := retry.WithMaxRetries(30, retry.NewConstant(1*time.Second))

	// Try to establish a connection to the database, with retries.
	var conn *pgx.Conn
	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		var err error
		conn, err = pgx.Connect(ctx, connectionURL.String())
		if err != nil {
			return retry.RetryableError(err)
		}
		if err := conn.Ping(ctx); err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed waiting for database container to be ready: %w", err)
	}

	// Run the migrations.
	if err := dbMigrate(connectionURL.String()); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Return the instance.
	return &TestInstance{
		pool:      pool,
		container: container,
		conn:      conn,
		url:       connectionURL,
	}, nil
}

// MustClose is like Close except it prints the error to stderr and calls os.Exit.
func (i *TestInstance) MustClose() error {
	if err := i.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return nil
}

// Close terminates the test database instance, cleaning up any resources.
func (i *TestInstance) Close() (retErr error) {
	// Do not attempt to close  things when there's nothing to close.
	if i.skipReason != "" {
		return
	}

	defer func() {
		if err := i.pool.Purge(i.container); err != nil {
			retErr = fmt.Errorf("failed to purge database container: %w", err)
			return
		}
	}()

	ctx := context.Background()
	if err := i.conn.Close(ctx); err != nil {
		retErr = fmt.Errorf("failed to close connection: %w", err)
		return
	}

	return
}

// NewDatabase creates a new database suitable for use in testing. It returns an
// established database connection and the configuration.
func (i *TestInstance) NewDatabase(tb testing.TB) (*DB, *Config) {
	tb.Helper()

	// Ensure we should actually create the database.
	if i.skipReason != "" {
		tb.Skip(i.skipReason)
	}

	// Clone the template database.
	newDatabaseName, err := i.clone()
	if err != nil {
		tb.Fatal(err)
	}

	// Build the new connection URL for the new database name. Query params are
	// dropped with ResolveReference, so we have to re-add disabling SSL over
	// localhost.
	connectionURL := i.url.ResolveReference(&url.URL{Path: newDatabaseName})
	connectionURL.RawQuery = "sslmode=disable"

	// Establish a connection to the database.
	ctx := context.Background()
	dbpool, err := pgxpool.Connect(ctx, connectionURL.String())
	if err != nil {
		tb.Fatalf("failed to connect to database %q: %s", newDatabaseName, err)
	}

	// Create the Go database instance.
	db := &DB{Pool: dbpool}

	// Close connection and delete database when done.
	tb.Cleanup(func() {
		ctx := context.Background()

		// Close connection first. It is an error to drop a database with active
		// connections.
		db.Close(ctx)

		// Drop the database to keep the container from running out of resources.
		q := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE);`, newDatabaseName)

		i.connLock.Lock()
		defer i.connLock.Unlock()

		if _, err := i.conn.Exec(ctx, q); err != nil {
			tb.Errorf("failed to drop database %q: %s", newDatabaseName, err)
		}
	})

	host, port, err := net.SplitHostPort(i.url.Host)
	if err != nil {
		tb.Errorf("failed to split host/port %q: %s", i.url.Host, err)
	}

	return db, &Config{
		Name:     newDatabaseName,
		User:     databaseUser,
		Host:     host,
		Port:     port,
		SSLMode:  "disable",
		Password: databasePassword,
	}
}

// clone creates a new database with a random name from the template instance.
func (i *TestInstance) clone() (string, error) {
	// Generate a random database name.
	name, err := randomDatabaseName()
	if err != nil {
		return "", fmt.Errorf("failed to generate random database name: %w", err)
	}

	// Setup context and create SQL command. Unfortunately we cannot use parameter
	// injection here as that's only valid for prepared statements, for which this
	// is not. Fortunately both inputs can be trusted in this case.
	ctx := context.Background()
	q := fmt.Sprintf(`CREATE DATABASE "%s" WITH TEMPLATE "%s";`, name, databaseName)

	// Unfortunately postgres does not allow parallel database creation from the
	// same template, so this is guarded with a lock.
	i.connLock.Lock()
	defer i.connLock.Unlock()

	// Clone the template database as the new random database name.
	if _, err := i.conn.Exec(ctx, q); err != nil {
		return "", fmt.Errorf("failed to clone template database: %w", err)
	}
	return name, nil
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

// postgresRepo returns the postgres container image name based on an
// environment variable, or the default value if the environment variable is
// unset.
func postgresRepo() (string, string, error) {
	ref := os.Getenv("CI_POSTGRES_IMAGE")
	if ref == "" {
		ref = defaultPostgresImageRef
	}

	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid reference for database container: %q", ref)
	}
	return parts[0], parts[1], nil
}

// randomDatabaseName returns a random database name.
func randomDatabaseName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
