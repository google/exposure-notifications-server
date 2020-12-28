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

// This package is the gRPC server for federation requests to send data to other federations servers.
package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.opencensus.io/plugin/ocgrpc"

	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/internal/federationout"
	"github.com/google/exposure-notifications-server/internal/pb/federation"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/logging"
	_ "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().
		With("build_id", buildinfo.BuildID).
		With("build_tag", buildinfo.BuildTag)
	ctx = logging.WithLogger(ctx, logger)

	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config federationout.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	federationServer := federationout.NewServer(env, &config)

	var sopts []grpc.ServerOption
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to create credentials: %w", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	if !config.AllowAnyClient {
		sopts = append(sopts, grpc.UnaryInterceptor(federationServer.(*federationout.Server).AuthInterceptor))
	}

	sopts = append(sopts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	grpcServer := grpc.NewServer(sopts...)
	federation.RegisterFederationServer(grpcServer, federationServer)

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("server.New: %w", err)
	}
	logger.Infof("listening on :%s", config.Port)

	return srv.ServeGRPC(ctx, grpcServer)
}
