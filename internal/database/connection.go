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

var (
	validSSLModes = []string{
		"disable",     // No SSL
		"require",     // Always SSL (skip verification)
		"verify-ca",   // Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
		"verify-full", // Always SSL (verify that the certification presented by
	}
)

type config struct {
	env       string
	part      string
	def       interface{}
	req       bool
	valid     []string
	writeFile bool
}

type DB struct {
	pool *pgxpool.Pool
}

// NewFromEnv sets up the database connections using the configuration in the
// process's environment variables. This should be called just once per server
// instance.

func NewFromEnv(ctx context.Context, env *Environment) (*DB, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Creating connection pool.")

	connStr, err := dbConnectionString(ctx, env)

	if err != nil {
		return nil, fmt.Errorf("invalid database config: %v", err)
	}

	pool, err := pgxpool.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %v", err)
	}

	return &DB{pool: pool}, nil
}

// Close releases database connections.
func (db *DB) Close(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Infof("Closing connection pool.")
	db.pool.Close()
}

// dbConnectionString builds a connection string suitable for the pgx Postgres driver, using the
// values of vars.
func dbConnectionString(ctx context.Context, env *Environment) (string, error) {
	vals := dbValues(env)
	var p []string
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(p, " "), nil
}

// dbURI builds a Postgres URI suitable for the lib/pq driver, which is used by
// github.com/golang-migrate/migrate.
func dbURI(env *Environment) string {
	return fmt.Sprintf("postgres://%s/%s?sslmode=disable&user=%s&password=%s&port=%s",
		env.Host, env.Name, env.User,
		url.QueryEscape(env.Password), url.QueryEscape(env.Port))
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

func dbValues(env *Environment) map[string]string {
	p := map[string]string{}
	setIfNotEmpty(p, "dbname", env.Name)
	setIfNotEmpty(p, "user", env.User)
	setIfNotEmpty(p, "host", env.Host)
	setIfNotEmpty(p, "port", env.Port)
	setIfNotEmpty(p, "sslmode", env.SSLMode)
	setIfPositive(p, "connect_timeout", env.ConnectionTimeout)
	setIfNotEmpty(p, "password", env.Password)
	setIfNotEmpty(p, "sslcert", env.SSLCert)
	setIfNotEmpty(p, "sslkey", env.SSLKey)
	setIfNotEmpty(p, "sslrootcert", env.SSLRootCert)
	setIfNotEmpty(p, "pool_min_conns", env.PoolMinConnections)
	setIfNotEmpty(p, "pool_max_conns", env.PoolMaxConnections)
	setIfPositiveDuration(p, "pool_max_conn_lifetime", env.PoolMaxConnLife)
	setIfPositiveDuration(p, "pool_max_conn_idle_time", env.PoolMaxConnIdle)
	setIfPositiveDuration(p, "pool_health_check_period", env.PoolHealthCheck)
	return p
}
