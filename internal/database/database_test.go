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
	"log"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/kelseyhightower/envconfig"

	// imported to register the postgres migration driver
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// imported to register the "file" source migration driver
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// imported to register the "postgres" database driver for migrate
)

var testDB *DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	if os.Getenv("DB_USER") != "" {
		var err error
		testDB, err = createTestDB(ctx)
		if err != nil {
			log.Fatalf("creating test DB: %v", err)
		}
	}
	os.Exit(m.Run())
}

// openTestDB connects to the Postgres server specified by the DB_XXX environment
// variables, creates an empty test database on it, and returns a *DB connected
// to that database.
func createTestDB(ctx context.Context) (*DB, error) {
	const testDBName = "exposure-server-test"

	// Connect to the default database to create the test database.
	var config Config
	err := envconfig.Process("dbtestEnvironment", &config)
	if err != nil {
		return nil, err
	}
	// Override DB name.
	config.Name = "postgres"
	db, err := NewFromEnv(ctx, &config)
	if err != nil {
		return nil, err
	}
	if err := db.createDatabase(ctx, testDBName); err != nil {
		return nil, err
	}
	db.Close(ctx)

	// Connect to the test database and create its schema by applying all migrations.
	config.Name = testDBName
	db, err = NewFromEnv(ctx, &config)
	if err != nil {
		return nil, err
	}
	const source = "file://../../migrations"
	uri := dbURI(&config)
	m, err := migrate.New(source, uri)
	if err != nil {
		return nil, err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		_, _ = m.Close()
		return nil, err
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return nil, srcErr
	}
	if dbErr != nil {
		return nil, dbErr
	}
	return db, nil
}

func (db *DB) createDatabase(ctx context.Context, name string) error {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q`, name)); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name))
	return err
}

func mustExec(t *testing.T, conn *pgxpool.Conn, stmt string, args ...interface{}) {
	_, err := conn.Exec(context.Background(), stmt, args...)
	if err != nil {
		t.Fatalf("executing %s: %v", stmt, err)
	}
}

func resetTestDB(t *testing.T) {
	ctx := context.Background()
	conn, err := testDB.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		TRUNCATE 
			FederationInQuery, FederationInSync, FederationOutAuthorization,
			Exposure, APIConfig,
			ExportConfig, ExportBatch, ExportFile
	`)
	if err != nil {
		t.Fatal(err)
	}
}
