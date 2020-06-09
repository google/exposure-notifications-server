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
	Password           string        `env:"DB_PASSWORD" json:"-"` // ignored by zap's JSON formatter
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
