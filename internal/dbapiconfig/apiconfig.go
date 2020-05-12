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

package dbapiconfig

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	provider "github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model/apiconfig"
)

// Compile-time check to assert Config implements the Provider interface.
var _ provider.Provider = (*dbConfigProvider)(nil)

type dbConfigProvider struct {
	db            *database.DB
	refreshPeriod time.Duration

	bypassSafetyNet   bool
	bypassDeviceCheck bool

	deviceCheckKeyID      string
	deviceCheckTeamID     string
	deviceCheckPrivateKey *ecdsa.PrivateKey

	mu           sync.RWMutex
	lastLoadTime time.Time
	cache        map[string]*apiconfig.APIConfig
}

type ConfigOpts struct {
	RefreshDuration       time.Duration `envconfig:"API_CONFIG_REFRESH" default:"5m"`
	BypassSafetyNet       bool          `envconfig:"BYPASS_SAFETYNET"`
	BypassDeviceCheck     bool          `envconfig:"BYPASS_DEVICECHECK"`
	DeviceCheckKeyID      string        `envconfig:"DEVICECHECK_KEY_ID"`
	DeviceCheckTeamID     string        `envconfig:"DEVICECHECK_TEAM_ID"`
	DeviceCheckPrivateKey string        `envconfig:"DEVICECHECK_PRIVATE_KEY"`
}

func NewConfigProvider(db *database.DB, opts *ConfigOpts) (provider.Provider, error) {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	cfg := &dbConfigProvider{
		db:                db,
		refreshPeriod:     opts.RefreshDuration,
		bypassSafetyNet:   opts.BypassSafetyNet,
		bypassDeviceCheck: opts.BypassDeviceCheck,
		deviceCheckKeyID:  opts.DeviceCheckKeyID,
		deviceCheckTeamID: opts.DeviceCheckTeamID,
		cache:             make(map[string]*apiconfig.APIConfig),
	}

	// TODO (mikehelmick): Load device check private key from secret store.

	if cfg.refreshPeriod <= 0 {
		return nil, fmt.Errorf("refresh duration must be a positive duration")
	}
	if cfg.refreshPeriod > time.Minute*5 {
		logger.Warnf("config refresh duration is > 5 minutes: %v", cfg.refreshPeriod)
	}

	if cfg.bypassSafetyNet {
		logger.Errorf("APIConfig provider will bypass SafetyNet verification for android devices. This should only be done in test environments.")
	}
	if cfg.bypassDeviceCheck {
		logger.Errorf("APIConfig provider will bypass DeviceCheck verification for iOS devices. This should only be done in test environments.")
	}

	return cfg, nil
}

func (c *dbConfigProvider) loadConfig(ctx context.Context) error {
	// In case multiple requests notice expiration simultaneously, only do it once.
	c.mu.Lock()
	defer c.mu.Unlock()

	// if the cache isn't expired, don't reload.
	if time.Since(c.lastLoadTime) < c.refreshPeriod {
		return nil
	}

	logger := logging.FromContext(ctx)

	configs, err := c.db.ReadAPIConfigs(ctx)
	if err != nil {
		return err
	}

	c.cache = make(map[string]*apiconfig.APIConfig)
	for _, apiConfig := range configs {
		apiConfig.BypassSafetyNet = c.bypassSafetyNet
		c.cache[apiConfig.AppPackageName] = apiConfig
	}
	logger.Info("loaded new APIConfig values")
	c.lastLoadTime = time.Now()
	return nil
}

func (c *dbConfigProvider) AppPkgConfig(ctx context.Context, appPkg string) (*apiconfig.APIConfig, error) {
	if err := c.loadConfig(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	cfg, ok := c.cache[appPkg]
	if !ok {
		logger := logging.FromContext(ctx)
		logger.Errorf("requested config for unconfigured app: %v", appPkg)
		return nil, nil
	}
	return cfg, nil
}
