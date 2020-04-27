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
	"cambio/pkg/logging"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	// mu guards the initialization of db
	mu   sync.Mutex
	pool *pgxpool.Pool

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
		{env: "DB_SSLCERT", part: "sslcert", def: ""},
		{env: "DB_SSLKEY", part: "sslkey", def: ""},
		{env: "DB_SSLROOTCERT", part: "sslrootcert", def: ""},
		{env: "DB_POOL_MAX_CONNS", part: "pool_max_conns", def: ""},
		{env: "DB_POOL_MIN_CONNS", part: "pool_min_conns", def: ""},
		{env: "DB_POOL_MAX_CONN_LIFETIME", part: "pool_max_conn_lifetime", def: time.Duration(0)},
		{env: "DB_POOL_MAX_CONN_IDLE_TIME", part: "pool_max_conn_idle_time", def: time.Duration(0)},
		{env: "DB_POOL_HEALTH_CHECK_PERIOD", part: "pool_health_check_period", def: time.Duration(0)},
	}
)

type config struct {
	env   string
	part  string
	def   interface{}
	req   bool
	valid []string
}

// Initialize sets up the database connections.
func Initialize(ctx context.Context) (cleanup func(context.Context), err error) {
	mu.Lock()
	defer mu.Unlock()
	if pool != nil {
		return nil, errors.New("connection pool already initialized")
	}

	logger := logging.FromContext(ctx)
	logger.Infof("Creating connection pool.")

	connStr, err := processEnv(ctx, configs)
	if err != nil {
		return nil, fmt.Errorf("invalid database config: %v", err)
	}

	pool, err = pgxpool.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %v", err)
	}

	return close, nil
}

// close releases pool connections.
func close(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()
	if pool != nil {
		logger := logging.FromContext(ctx)
		logger.Infof("Closing connection pool.")
		pool.Close()
	}
	pool = nil
}

// Connection returns a database connection. You must defer conn.Release() in order to return the connection to the pool.
func Connection(ctx context.Context) (*pgxpool.Conn, error) {
	return pool.Acquire(ctx)
}

func processEnv(ctx context.Context, configs []config) (string, error) {
	logger := logging.FromContext(ctx)
	var e, p []string
	for _, c := range configs {

		val := os.Getenv(c.env)
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
				p = append(p, fmt.Sprintf("%s=%s", c.part, s))
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
				p = append(p, fmt.Sprintf("%s=%d", c.part, i))
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
				p = append(p, fmt.Sprintf("%s=%s", c.part, d))
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
		return "", errors.New(errs)
	}

	return strings.Join(p, " "), nil
}
