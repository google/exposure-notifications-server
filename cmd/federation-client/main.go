package main

import (
	"cambio/pkg/pb"
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	timeout = 30 * time.Second
)

var (
	serverAddr    = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
	region        = flag.String("region", "US", "Comma-separated list of regions.")
	excludeRegion = flag.String("exclude_region", "", "Comma-separated list of regions to exclude.")
	lastTimestamp = flag.Int64("last_ts", 0, "Unix timestamp to start the query from.")
	cursor        = flag.String("cursor", "", "Cursor from previous partial response.")
)

func main() {
	flag.Parse()

	request := &pb.FederationFetchRequest{}
	if *region != "" {
		request.RegionIdentifiers = strings.Split(*region, ",")
	}
	if *excludeRegion != "" {
		request.ExcludeRegionIdentifiers = strings.Split(*excludeRegion, ",")
	}
	if *cursor != "" {
		request.FetchToken = *cursor
	}
	request.LastFetchResponseKeyTimestamp = *lastTimestamp

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
