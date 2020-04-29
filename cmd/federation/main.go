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
	"os"
	"time"

	"google.golang.org/grpc"

	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/pb"
	"cambio/pkg/serverenv"
)

const (
	timeoutEnvVar  = "FETCH_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	db, err := database.NewFromEnv(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	timeout := defaultTimeout
	if timeoutStr := os.Getenv(timeoutEnvVar); timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warnf("Failed to parse $%s value %q, using default.", timeoutEnvVar, timeoutStr)
			timeout = defaultTimeout
		}
	}
	logger.Infof("Using fetch timeout %v (override with $%s)", timeout, timeoutEnvVar)

	env := serverenv.New(ctx)
	grpcEndpoint := fmt.Sprintf(":%s", env.Port())
	logger.Infof("gRPC endpoint [%s]", grpcEndpoint)

	grpcServer := grpc.NewServer()
	pb.RegisterFederationServer(grpcServer, api.NewFederationServer(db, timeout))

	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting: gRPC Listener [%s]", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
