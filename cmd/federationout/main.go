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
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.opencensus.io/plugin/ocgrpc"

	"github.com/google/exposure-notifications-server/internal/federationout"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	var config federationout.Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		logger.Fatalf("setup.Setup: %v", err)
	}
	defer closer()

	server := federationout.NewServer(env, &config)

	var sopts []grpc.ServerOption
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	if !config.AllowAnyClient {
		sopts = append(sopts, grpc.UnaryInterceptor(server.(*federationout.Server).AuthInterceptor))
	}

	sopts = append(sopts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	grpcServer := grpc.NewServer(sopts...)
	pb.RegisterFederationServer(grpcServer, server)

	grpcEndpoint := ":" + config.Port
	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting federationout gRPC listener [%s]", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
