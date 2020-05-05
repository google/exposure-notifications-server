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

// This package is the gRPC server for federation requests from federations partners.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/google/exposure-notifications-server/internal/api/federation"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	timeoutEnvVar  = "FETCH_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	env, err := serverenv.New(ctx, serverenv.WithSecretManager)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	timeout := serverenv.ParseDuration(ctx, timeoutEnvVar, defaultTimeout)
	logger.Infof("Using fetch timeout %v (override with $%s)", timeout, timeoutEnvVar)

	grpcEndpoint := fmt.Sprintf(":%s", env.Port())
	logger.Infof("gRPC endpoint [%s]", grpcEndpoint)

	grpcServer := grpc.NewServer()
	pb.RegisterFederationServer(grpcServer, federation.NewServer(db, timeout))

	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting: gRPC Listener [%s]", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
