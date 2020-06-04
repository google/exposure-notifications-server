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
	"errors"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.opencensus.io/plugin/ocgrpc"

	"github.com/google/exposure-notifications-server/internal/federationout"
	"github.com/google/exposure-notifications-server/internal/interrupt"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx, done := interrupt.Context()
	defer done()

	if err := realMain(ctx); err != nil {
		logger := logging.FromContext(ctx)
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

	server := federationout.NewServer(env, &config)

	var sopts []grpc.ServerOption
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to create credentials: %w", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	if !config.AllowAnyClient {
		sopts = append(sopts, grpc.UnaryInterceptor(server.(*federationout.Server).AuthInterceptor))
	}

	sopts = append(sopts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	grpcServer := grpc.NewServer(sopts...)
	pb.RegisterFederationServer(grpcServer, server)

	addr := fmt.Sprintf(":%s", config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go func(ctx context.Context) {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger := logging.FromContext(ctx)
			logger.Errorf("grpc serving error: %v", err)
		}
	}(ctx)
	logger.Infof("listening on :%s", config.Port)

	// Wait for cancel or interrupt
	<-ctx.Done()

	// Shutdown
	logger.Info("received shutdown")
	grpcServer.GracefulStop()

	logger.Info("shutdown complete")
	return nil

}
