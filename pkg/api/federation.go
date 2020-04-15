package api

import (
	"cambio/pkg/pb"
	"context"
)

// FederationService implements gRPC methods for server-to-server interactions.
type FederationService struct{}

// NewFederationService builds a new FederationService.
func NewFederationService() *FederationService {
	return &FederationService{}
}

// Fetch implements the FederationService Fetch endpoint.
func (s *FederationService) Fetch(ctx context.Context, in *pb.FederationFetchRequest) (*pb.FederationFetchResponse, error) {
	return &pb.FederationFetchResponse{}, nil
}
