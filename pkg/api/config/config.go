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

package config

import (
	"context"
	"os"
	"sync"
	"time"

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
)

const (
	defaultRefreshPeriod = time.Minute
)

type Config struct {
	mu            sync.RWMutex
	lastLoadTime  time.Time
	cache         map[string]*model.APIConfig
	refreshPeriod time.Duration
}

func New() *Config {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	cfg := &Config{
		cache:         make(map[string]*model.APIConfig),
		refreshPeriod: defaultRefreshPeriod,
	}

	if ds := os.Getenv("CONFIG_REFRESH_DURATION"); ds != "" {
		if d, err := time.ParseDuration(ds); err != nil {
			logger.Info("CONFIG_REFRESH_DURATION parse error: %v", defaultRefreshPeriod)
		} else {
			cfg.refreshPeriod = d
		}
	}

	if cfg.refreshPeriod > time.Minute*5 {
		logger.Warn("config refresh duration is > 5 minutes: %v", cfg.refreshPeriod)
	}
	return cfg
}

func (c *Config) loadConfig(ctx context.Context) error {
	// In case multiple requests notice expiration simultaneously, only do it once.
	c.mu.Lock()
	defer c.mu.Unlock()

	// if the cache isn't expired, don't reload.
	if time.Since(c.lastLoadTime) < c.refreshPeriod {
		return nil
	}

	logger := logging.FromContext(ctx)

	configs, err := database.ReadAPIConfigs(ctx)
	if err != nil {
		// This will exit the server. Without a valid config, we cannot process
		// requests.
		// TODO(mikehelmick) stable fallbacks
		logger.Fatalf("error loading APIConfig: %v", err)
		return err
	}

	c.cache = make(map[string]*model.APIConfig)
	for _, apiConfig := range configs {
		c.cache[apiConfig.AppPackageName] = apiConfig
	}
	logger.Info("loaded new APIConfig values")
	c.lastLoadTime = time.Now()

	return nil
}

func (c *Config) AppPkgConfig(ctx context.Context, appPkg string) *model.APIConfig {
	c.loadConfig(ctx)

	c.mu.RLock()
	defer c.mu.RUnlock()

	cfg, ok := c.cache[appPkg]
	if !ok {
		logger := logging.FromContext(ctx)
		logger.Errorf("requested config for unconfigured app: %v", appPkg)
		return nil
	}
	return cfg
}
