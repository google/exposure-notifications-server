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
	"time"
)

type Config struct {
	Name               string        `env:"DB_NAME"`
	User               string        `env:"DB_USER"`
	Host               string        `env:"DB_HOST, default=localhost"`
	Port               string        `env:"DB_PORT, default=5432"`
	SSLMode            string        `env:"DB_SSLMODE, default=require"`
	ConnectionTimeout  int           `env:"DB_CONNECT_TIMEOUT"`
	Password           string        `env:"DB_PASSWORD"`
	SSLCertPath        string        `env:"DB_SSLCERT"`
	SSLKeyPath         string        `env:"DB_SSLKEY"`
	SSLRootCertPath    string        `env:"DB_SSLROOTCERT"`
	PoolMinConnections string        `env:"DB_POOL_MIN_CONNS"`
	PoolMaxConnections string        `env:"DB_POOL_MAX_CONNS"`
	PoolMaxConnLife    time.Duration `env:"DB_POOL_MAX_CONN_LIFETIME"`
	PoolMaxConnIdle    time.Duration `env:"DB_POOL_MAX_CONN_IDLE_TIME"`
	PoolHealthCheck    time.Duration `env:"DB_POOL_HEALTH_CHECK_PERIOD"`
}

func (c *Config) DatabaseConfig() *Config {
	return c
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		Host:    "localhost",
		Port:    "5432",
		SSLMode: "require",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Name:               "dbname",
		User:               "dbuser",
		Host:               "https://dbhost",
		Port:               "5555",
		SSLMode:            "verify-ca",
		ConnectionTimeout:  30,
		Password:           "abcd1234",
		SSLCertPath:        "/var/sslcert",
		SSLKeyPath:         "/var/sslkey",
		SSLRootCertPath:    "/var/sslrootcert",
		PoolMinConnections: "5",
		PoolMaxConnections: "50",
		PoolMaxConnLife:    5 * time.Minute,
		PoolMaxConnIdle:    10 * time.Minute,
		PoolHealthCheck:    15 * time.Minute,
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	return map[string]string{
		"DB_NAME":                     "dbname",
		"DB_USER":                     "dbuser",
		"DB_HOST":                     "https://dbhost",
		"DB_PORT":                     "5555",
		"DB_SSLMODE":                  "verify-ca",
		"DB_CONNECT_TIMEOUT":          "30",
		"DB_PASSWORD":                 "abcd1234",
		"DB_SSLCERT":                  "/var/sslcert",
		"DB_SSLKEY":                   "/var/sslkey",
		"DB_SSLROOTCERT":              "/var/sslrootcert",
		"DB_POOL_MIN_CONNS":           "5",
		"DB_POOL_MAX_CONNS":           "50",
		"DB_POOL_MAX_CONN_LIFETIME":   "5m",
		"DB_POOL_MAX_CONN_IDLE_TIME":  "10m",
		"DB_POOL_HEALTH_CHECK_PERIOD": "15m",
	}
}

// TestConfigOverridden returns a configuration with non-default values set. It
// should only be used for testing.
func TestConfigOverridden() *Config {
	return &Config{
		Name:               "dbname2",
		User:               "dbuser2",
		Host:               "https://dbhost2",
		Port:               "5556",
		SSLMode:            "verify-full",
		ConnectionTimeout:  60,
		Password:           "efgh5678",
		SSLCertPath:        "/var/sslcert2",
		SSLKeyPath:         "/var/sslkey2",
		SSLRootCertPath:    "/var/sslrootcert2",
		PoolMinConnections: "10",
		PoolMaxConnections: "100",
		PoolMaxConnLife:    1 * time.Minute,
		PoolMaxConnIdle:    10 * time.Minute,
		PoolHealthCheck:    100 * time.Minute,
	}
}
