// This package is a CLI tool for calling the gRPC federation server, for manual testing.
package main

import (
	cflag "cambio/pkg/flag"
	"cambio/pkg/pb"
	"context"
	"flag"
	"log"
	"time"

	"google.golang.org/grpc"
)

const (
	timeout = 30 * time.Second
)

var (
	serverAddr    = flag.String("server-addr", "localhost:10000", "The server address in the format of host:port")
	lastTimestamp = flag.String("last-timestamp", "", "The last timestamp (RFC3339) to set; queries start from this point and go forward.")
	cursor        = flag.String("cursor", "", "Cursor from previous partial response.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	lastTime, err := time.Parse(time.RFC3339, *lastTimestamp)
	if err != nil {
		log.Fatalf("failed to parse --last-timestamp (use RFC3339): %v", err)
	}

	request := &pb.FederationFetchRequest{
		RegionIdentifiers:             includeRegions,
		ExcludeRegionIdentifiers:      excludeRegions,
		FetchToken:                    *cursor,
		LastFetchResponseKeyTimestamp: lastTime.Unix(),
	}

	// See https://github.com/grpc/grpc-go/blob/master/examples/route_guide/client/client.go
	conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial %s: %v", *serverAddr, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	response, err := pb.NewFederationClient(conn).Fetch(ctx, request)
	if err != nil {
		log.Fatalf("Error calling fetch: %v", err)
	}

	log.Printf("Result:\n%#v\n", response)
}
