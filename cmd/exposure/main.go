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

// This package is the primary infected keys upload service.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/api/handlers"
	"github.com/google/exposure-notifications-server/internal/api/publish"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/ios"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	minPublishDurationEnv     = "MIN_PUBLISH_DURATION"
	defaultMinPublishDiration = 5 * time.Second

	bypassDeviceCheckEnv = "BYPASS_DEVICECHECK"
	bypassSafetyNetEnv   = "BYPASS_SAFETYNET"

	deviceCheckPrivateKeyEnv = "DEVICECHECK_PRIVATE_KEY"
	deviceCheckKeyIDEnv      = "DEVICECHECK_KEY_ID"
	deviceCheckTeamIDEnv     = "DEVICECHECK_TEAM_ID"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	// It is possible to install a different secret management system here that conforms to secrets.SecretManager{}
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}
	env := serverenv.New(ctx,
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext))

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	cfg := config.New(db)

	bypassDeviceCheck := serverenv.ParseBool(ctx, bypassDeviceCheckEnv, false)
	if bypassDeviceCheck {
		logger.Warn("iOS DeviceCheck verification is bypassed. Do not bypass " +
			"DeviceCheck verification in production environments!")
		cfg.BypassDeviceCheck()
	} else {
		keyID, err := env.ResolveSecretEnv(ctx, deviceCheckKeyIDEnv)
		if err != nil {
			logger.Fatalf("failed to resolve %q: %v", deviceCheckKeyIDEnv, err)
		}
		cfg.SetDeviceCheckKeyID(keyID)

		teamID, err := env.ResolveSecretEnv(ctx, deviceCheckTeamIDEnv)
		if err != nil {
			logger.Fatalf("failed to resolve %q: %v", deviceCheckTeamIDEnv, err)
		}
		cfg.SetDeviceCheckTeamID(teamID)

		privateKeyStr, err := env.ResolveSecretEnv(ctx, deviceCheckPrivateKeyEnv)
		if err != nil {
			logger.Fatalf("failed to resolve %q: %v", deviceCheckPrivateKeyEnv, err)
		}
		privateKey, err := ios.ParsePrivateKey(privateKeyStr)
		if err != nil {
			logger.Fatalf("bad device check private key: %v", err)
		}
		cfg.SetDeviceCheckPrivateKey(privateKey)
	}

	bypassSafetyNet := serverenv.ParseBool(ctx, bypassSafetyNetEnv, false)
	if bypassSafetyNet {
		logger.Warn("Android SafetyNet verification is bypassed. Do not bypass " +
			"SafetyNet verification in production environments!")
		cfg.BypassSafetyNet()
	}

	minLatency := serverenv.ParseDuration(ctx, minPublishDurationEnv, defaultMinPublishDiration)
	logger.Infof("Request minimum latency is: %v", minLatency.String())

	handler, err := publish.NewHandler(ctx, db, cfg, env)
	if err != nil {
		logger.Fatalf("unable to create publish handler: %v", err)
	}
	http.Handle("/", handlers.WithMinimumLatency(minLatency, handler))
	logger.Info("starting exposure server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port), nil))
}
