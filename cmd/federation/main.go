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
)

const (
	portEnvVar     = "PORT"
	defaultPort    = "8080"
	timeoutEnvVar  = "FETCH_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := database.Initialize(); err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}

	port := os.Getenv(portEnvVar)
	if port == "" {
		port = defaultPort
	}
	logger.Infof("Using port %s (override with $%s)", port, portEnvVar)

	timeout := defaultTimeout
	timeoutStr := os.Getenv(timeoutEnvVar)
	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warnf("Failed to parse $%s value %q, using default.", timeoutEnvVar, timeoutStr)
			timeout = defaultTimeout
		}
	}
	logger.Infof("Using fetch timeout %v (override with $%s)", timeout, timeoutEnvVar)

	grpcEndpoint := fmt.Sprintf(":%s", port)
	logger.Infof("gRPC endpoint [%s]", grpcEndpoint)

	grpcServer := grpc.NewServer()
	pb.RegisterFederationServer(grpcServer, api.NewFederationServer(timeout))

	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting: gRPC Listener [%s]", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
