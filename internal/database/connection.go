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
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/serverenv"

	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	defaultHost               = "localhost"
	defaultPort               = 5432
	defaultSSLMode            = "required"
	defaultMaxIdleConnections = 10
	defaultMaxOpenConnections = 10
)

var (
	validSSLModes = []string{
		"disable",     // No SSL
		"require",     // Always SSL (skip verification)
		"verify-ca",   // Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
		"verify-full", // Always SSL (verify that the certification presented by
	}

	configs = []config{
		{env: "DB_DBNAME", part: "dbname", def: "", req: true},
		{env: "DB_USER", part: "user", def: "", req: true},
		{env: "DB_HOST", part: "host", def: defaultHost},
		{env: "DB_PORT", part: "port", def: defaultPort},
		{env: "DB_SSLMODE", part: "sslmode", def: defaultSSLMode, valid: validSSLModes},
		{env: "DB_CONNECT_TIMEOUT", part: "connect_timeout", def: 0},
		{env: "DB_PASSWORD", part: "password", def: ""},
		{env: "DB_SSLCERT", part: "sslcert", def: "", writeFile: true},
		{env: "DB_SSLKEY", part: "sslkey", def: "", writeFile: true},
		{env: "DB_SSLROOTCERT", part: "sslrootcert", def: "", writeFile: true},
		{env: "DB_POOL_MAX_CONNS", part: "pool_max_conns", def: ""},
		{env: "DB_POOL_MIN_CONNS", part: "pool_min_conns", def: ""},
		{env: "DB_POOL_MAX_CONN_LIFETIME", part: "pool_max_conn_lifetime", def: time.Duration(0)},
		{env: "DB_POOL_MAX_CONN_IDLE_TIME", part: "pool_max_conn_idle_time", def: time.Duration(0)},
		{env: "DB_POOL_HEALTH_CHECK_PERIOD", part: "pool_health_check_period", def: time.Duration(0)},
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

func NewFromEnv(ctx context.Context, env *serverenv.ServerEnv) (*DB, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Creating connection pool.")

	connStr, err := dbConnectionString(ctx, configs, env)

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
func dbConnectionString(ctx context.Context, configs []config, env *serverenv.ServerEnv) (string, error) {
	vals, err := dbValues(ctx, configs, env)
	if err != nil {
		return "", err
	}
	var p []string
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(p, " "), nil
}

// dbURI builds a Postgres URI suitable for the lib/pq driver, which is used by
// github.com/golang-migrate/migrate.
func dbURI(ctx context.Context, configs []config, env *serverenv.ServerEnv) (string, error) {
	vals, err := dbValues(ctx, configs, env)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("postgres://%s/%s?sslmode=disable&user=%s&password=%s&port=%s",
		vals["host"], vals["dbname"], url.QueryEscape(vals["user"]),
		url.QueryEscape(vals["password"]), url.QueryEscape(vals["port"])), nil
}

func getenv(ctx context.Context, name string, toFile bool, env *serverenv.ServerEnv) (string, error) {
	if env == nil {
		return os.Getenv(name), nil
	}
	if toFile {
		return env.WriteSecretToFile(ctx, name)
	}
	return env.ResolveSecretEnv(ctx, name)
}

func dbValues(ctx context.Context, configs []config, env *serverenv.ServerEnv) (map[string]string, error) {
	logger := logging.FromContext(ctx)
	var e []string
	p := map[string]string{}
	for _, c := range configs {

		val, err := getenv(ctx, c.env, c.writeFile, env)
		if err != nil {
			e = append(e, fmt.Sprintf("$%s secret access: %v", c.env, err))
			continue
		}
		if c.req && val == "" {
			e = append(e, fmt.Sprintf("$%s is required", c.env))
			continue
		}

		switch def := c.def.(type) {

		case string:
			s := def
			if val == "" {
				if def != "" {
					logger.Infof("$%s not specified, using default value %q", c.env, def)
				}
			} else {
				s = val
				if len(c.valid) > 0 {
					isValid := false
					for _, enum := range c.valid {
						if enum == s {
							isValid = true
							break
						}
					}
					if !isValid {
						e = append(e, fmt.Sprintf("$%s %q must be one of %v", c.env, val, c.valid))
					}
				}
			}
			if s != "" {
				p[c.part] = s
			}

		case int:
			i := def
			if val == "" {
				if def != 0 {
					logger.Infof("$%s not specified, using default value %d", c.env, def)
				}
			} else {
				var err error
				i, err = strconv.Atoi(val)
				if err != nil || i < 0 {
					e = append(e, fmt.Sprintf("$%s %q must be a positive integer", c.env, val))
				}
			}
			if i != 0 {
				p[c.part] = fmt.Sprintf("%d", i)
			}

		case time.Duration:
			d := def
			if val == "" {
				if def != 0 {
					logger.Infof("$%s not specified, using default value %s", c.env, def)
				}
			} else {
				var err error
				d, err = time.ParseDuration(val)
				if err != nil {
					e = append(e, fmt.Sprintf("$%s %q is an invalid duration: %v", c.env, val, err))
				}
			}
			if d != 0 {
				p[c.part] = d.String()
			}

		default:
			e = append(e, fmt.Sprintf("unknown type %T", c.def))
		}
	}

	if len(e) > 0 {
		errs := "errors:\n"
		for _, item := range e {
			errs += fmt.Sprintf("  - %s\n", item)
		}
		return nil, errors.New(errs)
	}
	if len(p) == 0 {
		p = nil
	}
	return p, nil
}
