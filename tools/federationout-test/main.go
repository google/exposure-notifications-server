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

// This package is a CLI tool for calling the gRPC federationout server, for manual testing.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"time"

	"github.com/google/exposure-notifications-server/internal/federationin"
	cflag "github.com/google/exposure-notifications-server/internal/flag"
	"github.com/google/exposure-notifications-server/internal/pb"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"go.opencensus.io/plugin/ocgrpc"
)

const (
	timeout = 30 * time.Second
)

var (
	// See https://github.com/grpc/grpc-go/blob/master/examples/route_guide/client/client.go
	serverAddr    = flag.String("server-addr", "localhost:8080", "The server address in the format of host:port")
	audience      = flag.String("audience", federationin.DefaultAudience, "The OIDC audience to use when creating client tokens.")
	lastTimestamp = flag.String("last-timestamp", "", "The last timestamp (RFC3339) to set; queries start from this point and go forward.")
	cursor        = flag.String("cursor", "", "Cursor from previous partial response.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	var lastTime time.Time
	var err error
	if *lastTimestamp != "" {
		lastTime, err = time.Parse(time.RFC3339, *lastTimestamp)
		if err != nil {
			log.Fatalf("Failed to parse --last-timestamp (use RFC3339): %v", err)
		}
	}

	if *audience != "" && !federationin.ValidAudienceRegexp.MatchString(*audience) {
		log.Fatalf("--audience %q must match %s", *audience, federationin.ValidAudienceStr)
	}

	request := &pb.FederationFetchRequest{
		RegionIdentifiers:             includeRegions,
		ExcludeRegionIdentifiers:      excludeRegions,
		NextFetchToken:                *cursor,
		LastFetchResponseKeyTimestamp: lastTime.Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithTransportCredentials(creds),
	}

	idTokenSource, err := idtoken.NewTokenSource(ctx, *audience)
	if err != nil {
		log.Fatalf("Failed to create token source: %v", err)
	}
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(oauth.TokenSource{
		TokenSource: idTokenSource,
	}))

	conn, err := grpc.Dial(*serverAddr, dialOpts...)
	if err != nil {
		log.Fatalf("Failed to dial %s: %v", *serverAddr, err)
	}
	defer conn.Close()

	total := 0
	response, err := pb.NewFederationClient(conn).Fetch(ctx, request)
	if err != nil {
		log.Fatalf("Error calling fetch: %v", err)
	}

	for _, ctr := range response.Response {
		log.Printf("%v", ctr.RegionIdentifiers)
		for _, cti := range ctr.ContactTracingInfo {
			log.Printf("    (%v)", cti.TransmissionRisk)
			for _, dk := range cti.ExposureKeys {
				total++
				log.Printf("        {[bytes] number %d count %d}", dk.IntervalNumber, dk.IntervalCount)
			}
		}
	}
	log.Printf("partialResponse: %t", response.PartialResponse)
	log.Printf("nextFetchToken:  %s", response.NextFetchToken)
	log.Printf("fetchResponseKeyTimestamp: %d", response.FetchResponseKeyTimestamp)
	log.Printf("number records:  %d", total)
}
