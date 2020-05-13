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
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/google/exposure-notifications-server/internal/api/federation"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/secrets"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	// It is possible to install a different secret management system here that conforms to secrets.SecretManager{}
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	var config federation.Config
	if err := envconfig.Process(ctx, &config, sm); err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	db, err := database.NewFromEnv(ctx, config.Database)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	server := federation.NewServer(db, config)

	var sopts []grpc.ServerOption
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	if !config.AllowAnyClient {
		sopts = append(sopts, grpc.UnaryInterceptor(server.(*federation.Server).AuthInterceptor))
	}

	grpcServer := grpc.NewServer(sopts...)
	pb.RegisterFederationServer(grpcServer, server)

	grpcEndpoint := ":" + config.Port
	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting: gRPC Listener [%s]", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
