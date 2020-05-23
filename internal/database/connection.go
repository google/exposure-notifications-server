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

// Package database is a facade over the data storage layer.
package database

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"

	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

// NewFromEnv sets up the database connections using the configuration in the
// process's environment variables. This should be called just once per server
// instance.

func NewFromEnv(ctx context.Context, config *Config) (*DB, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Creating connection pool.")

	connStr, err := dbConnectionString(ctx, config)

	if err != nil {
		return nil, fmt.Errorf("invalid database config: %v", err)
	}

	pool, err := pgxpool.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %v", err)
	}

	return &DB{Pool: pool}, nil
}

// Close releases database connections.
func (db *DB) Close(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Infof("Closing connection pool.")
	db.Pool.Close()
}

// dbConnectionString builds a connection string suitable for the pgx Postgres driver, using the
// values of vars.
func dbConnectionString(ctx context.Context, config *Config) (string, error) {
	vals := dbValues(config)
	var p []string
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(p, " "), nil
}

// dbURI builds a Postgres URI suitable for the lib/pq driver, which is used by
// github.com/golang-migrate/migrate.
func DbURI(config *Config) string {
	return fmt.Sprintf("postgres://%s/%s?sslmode=disable&user=%s&password=%s&port=%s",
		config.Host, config.Name, config.User,
		url.QueryEscape(config.Password), url.QueryEscape(config.Port))
}

func setIfNotEmpty(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}

func setIfPositive(m map[string]string, key string, val int) {
	if val > 0 {
		m[key] = fmt.Sprintf("%d", val)
	}
}

func setIfPositiveDuration(m map[string]string, key string, d time.Duration) {
	if d > 0 {
		m[key] = d.String()
	}
}

func dbValues(config *Config) map[string]string {
	p := map[string]string{}
	setIfNotEmpty(p, "dbname", config.Name)
	setIfNotEmpty(p, "user", config.User)
	setIfNotEmpty(p, "host", config.Host)
	setIfNotEmpty(p, "port", config.Port)
	setIfNotEmpty(p, "sslmode", config.SSLMode)
	setIfPositive(p, "connect_timeout", config.ConnectionTimeout)
	setIfNotEmpty(p, "password", config.Password)
	setIfNotEmpty(p, "sslcert", config.SSLCertPath)
	setIfNotEmpty(p, "sslkey", config.SSLKeyPath)
	setIfNotEmpty(p, "sslrootcert", config.SSLRootCertPath)
	setIfNotEmpty(p, "pool_min_conns", config.PoolMinConnections)
	setIfNotEmpty(p, "pool_max_conns", config.PoolMaxConnections)
	setIfPositiveDuration(p, "pool_max_conn_lifetime", config.PoolMaxConnLife)
	setIfPositiveDuration(p, "pool_max_conn_idle_time", config.PoolMaxConnIdle)
	setIfPositiveDuration(p, "pool_health_check_period", config.PoolHealthCheck)
	return p
}
