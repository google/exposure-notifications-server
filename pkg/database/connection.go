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
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"

	pgx "github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

// NewFromEnv sets up the database connections using the configuration in the
// process's environment variables. This should be called just once per server
// instance.
func NewFromEnv(ctx context.Context, cfg *Config) (*DB, error) {
	pgxConfig, err := pgxpool.ParseConfig(dbDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// BeforeAcquire is called before before a connection is acquired from the
	// pool. It must return true to allow the acquision or false to indicate that
	// the connection should be destroyed and a different connection should be
	// acquired.
	pgxConfig.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		// Ping the connection to see if it is still valid. Ping returns an error if
		// it fails.
		return conn.Ping(ctx) == nil
	}

	pool, err := pgxpool.ConnectConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close releases database connections.
func (db *DB) Close(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Infof("Closing connection pool.")
	db.Pool.Close()
}

// dbDSN builds a connection string suitable for the pgx Postgres driver, using
// the values of vars.
func dbDSN(cfg *Config) string {
	vals := dbValues(cfg)
	p := make([]string, 0, len(vals))
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(p, " ")
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

func dbValues(cfg *Config) map[string]string {
	p := map[string]string{}
	setIfNotEmpty(p, "dbname", cfg.Name)
	setIfNotEmpty(p, "user", cfg.User)
	setIfNotEmpty(p, "host", cfg.Host)
	setIfNotEmpty(p, "port", cfg.Port)
	setIfNotEmpty(p, "sslmode", cfg.SSLMode)
	setIfPositive(p, "connect_timeout", cfg.ConnectionTimeout)
	setIfNotEmpty(p, "password", cfg.Password)
	setIfNotEmpty(p, "sslcert", cfg.SSLCertPath)
	setIfNotEmpty(p, "sslkey", cfg.SSLKeyPath)
	setIfNotEmpty(p, "sslrootcert", cfg.SSLRootCertPath)
	setIfNotEmpty(p, "pool_min_conns", cfg.PoolMinConnections)
	setIfNotEmpty(p, "pool_max_conns", cfg.PoolMaxConnections)
	setIfPositiveDuration(p, "pool_max_conn_lifetime", cfg.PoolMaxConnLife)
	setIfPositiveDuration(p, "pool_max_conn_idle_time", cfg.PoolMaxConnIdle)
	setIfPositiveDuration(p, "pool_health_check_period", cfg.PoolHealthCheck)
	return p
}
