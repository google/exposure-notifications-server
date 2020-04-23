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
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// Register postgres.
	_ "github.com/lib/pq"
)

const (
	databaseType              = "postgres"
	defaultHost               = "localhost"
	defaultPort               = "5432"
	defaultSSLMode            = "required"
	defaultMaxIdleConnections = 10
	defaultMaxOpenConnections = 10
)

var (
	// mu guards the initialization of db
	mu sync.Mutex
	db *sql.DB

	validSSLModes = []string{
		"disable",     // No SSL
		"require",     // Always SSL (skip verification)
		"verify-ca",   // Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
		"verify-full", // Always SSL (verify that the certification presented by
	}
)

// Initialize sets up the database connections.
func Initialize() error {
	mu.Lock()
	defer mu.Unlock()
	if db != nil {
		return errors.New("database connection already initialized")
	}

	ctx := context.Background()
	logger := logging.FromContext(ctx)

	var e []string
	dbname := os.Getenv("DB_DBNAME")
	if dbname == "" {
		e = append(e, "$DB_DBNAME is required")
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		e = append(e, "$DB_USER is required")
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		logger.Infof("$DB_HOST not found. Using default %q", defaultHost)
		host = defaultHost
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		logger.Infof("$DB_PORT not found. Using default %q", defaultPort)
		port = defaultPort
	}
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		logger.Infof("$DB_SSLMODE not found. Using default %q", defaultSSLMode)
		sslmode = defaultSSLMode
	} else {
		valid := false
		for _, v := range validSSLModes {
			if sslmode == v {
				valid = true
				break
			}
		}
		if !valid {
			e = append(e, fmt.Sprintf("$DB_SSLMODE %q is not valid; must be one of %v", sslmode, validSSLModes))
		}
	}
	connectTimeout := os.Getenv("DB_CONNECT_TIMEOUT")
	if connectTimeout != "" {
		dur, err := time.ParseDuration(connectTimeout)
		if err != nil {
			e = append(e, fmt.Sprintf("$DB_CONNECT_TIMEOUT parsing failed: %v", err))
		}
		connectTimeout = string(int64(dur.Seconds()))
	}
	password := os.Getenv("DB_PASSWORD") // TODO(jasonco): move to secrets, or rely on certificates
	sslcert := os.Getenv("DB_SSLCERT")
	sslkey := os.Getenv("DB_SSLKEY")
	sslrootcert := os.Getenv("DB_SSLROOTCERT")

	maxIdle := 0
	maxIdleStr := os.Getenv("DB_MAX_IDLE_CONNS")
	if maxIdleStr != "" {
		v, err := strconv.Atoi(maxIdleStr)
		if err != nil || v < 0 {
			e = append(e, fmt.Sprintf("$DB_MAX_IDLE_CONNS %q must be a non-negative integer", maxIdleStr))
		}
		maxIdle = v
	}

	maxOpen := 0
	maxOpenStr := os.Getenv("DB_MAX_OPEN_CONNS")
	if maxOpenStr != "" {
		v, err := strconv.Atoi(maxOpenStr)
		if err != nil || v < 0 {
			e = append(e, fmt.Sprintf("$DB_MAX_OPEN_CONNS %q must be a non-negative integer", maxOpenStr))
		}
		maxOpen = v
	}

	var maxLifetime time.Duration
	maxLifetimeStr := os.Getenv("DB_CONN_MAX_LIFETIME")
	if maxLifetimeStr != "" {
		dur, err := time.ParseDuration(maxLifetimeStr)
		if err != nil {
			e = append(e, fmt.Sprintf("$DB_CONN_MAX_LIFETIME parsing failed: %v", err))
		}
		maxLifetime = dur
	}

	if len(e) > 0 {
		msg := "database config errors:\n"
		for _, s := range e {
			msg += fmt.Sprintf("  - %s\n", s)
		}
		return errors.New(msg)
	}

	// See https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters
	parts := []string{
		"dbname=" + dbname,
		"user=" + user,
		"host=" + host,
		"port=" + port,
		"sslmode=" + sslmode,
		// "fallback_application_name="+fallbackApplicationName,
	}
	if password != "" {
		parts = append(parts, "password="+password)
	}
	if connectTimeout != "" {
		parts = append(parts, "connect_timeout="+connectTimeout)
	}
	if sslcert != "" {
		parts = append(parts, "sslcert="+connectTimeout)
	}
	if sslkey != "" {
		parts = append(parts, "sslkey="+connectTimeout)
	}
	if sslrootcert != "" {
		parts = append(parts, "sslrootcert="+connectTimeout)
	}

	connStr := strings.Join(parts, " ")

	var err error
	db, err = sql.Open(databaseType, connStr)
	if err != nil {
		return fmt.Errorf("opening database: %v", err)
	}

	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxLifetime != 0 {
		db.SetConnMaxLifetime(maxLifetime)
	}

	return nil
}

// Connection returns a database connection.
func Connection(ctx context.Context) (*sql.Conn, error) {
	return db.Conn(ctx)
}
