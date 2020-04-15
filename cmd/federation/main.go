package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	"cambio/pkg/api"
	"cambio/pkg/logging"
	"cambio/pkg/pb"
)

const (
	defaultPort = "8080"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	grpcEndpoint := fmt.Sprintf(":%s", port)
	logger.Infof("gRPC endpoint [%s]", grpcEndpoint)

	grpcServer := grpc.NewServer()
	pb.RegisterFederationServer(grpcServer, api.NewFederationService())

	listen, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
	logger.Infof("Starting: gRPC Listener [%s]\n", grpcEndpoint)
	log.Fatal(grpcServer.Serve(listen))
}
