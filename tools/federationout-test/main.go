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
	"github.com/google/exposure-notifications-server/internal/pb/federation"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"go.opencensus.io/plugin/ocgrpc"
)

const (
	timeout = 60 * time.Second
)

var (
	// See https://github.com/grpc/grpc-go/blob/master/examples/route_guide/client/client.go
	serverAddr           = flag.String("server-addr", "localhost:8080", "The server address in the format of host:port")
	audience             = flag.String("audience", federationin.DefaultAudience, "The OIDC audience to use when creating client tokens.")
	lastTimestamp        = flag.Int64("last-timestamp", 0, "The last timestamp, UTC seconds since the Epoch")
	cursor               = flag.String("cursor", "", "Cursor from previous partial response.")
	lastRevisedTimestamp = flag.Int64("last-revised-timestamp", 0, "The last revised timestamp, UTC seconds since the Epoch")
	revisedCursor        = flag.String("revised-cursor", "", "Cursor for revised keys from previous partial response")
	onlyTravelers        = flag.Bool("only-travelers", false, "only include travelers in fetch")
	onlyLocalProvenance  = flag.Bool("only-local-provenance", true, "only inclue local provenance keys")

	skipAuth = flag.Bool("skip-auth", false, "skip all auth and TLS")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	lastTime := time.Unix(*lastTimestamp, 0)
	lastRevisedTime := time.Unix(*lastRevisedTimestamp, 0)

	if *audience != "" && !federationin.ValidAudienceRegexp.MatchString(*audience) {
		log.Fatalf("--audience %q must match %s", *audience, federationin.ValidAudienceStr)
	}

	request := &federation.FederationFetchRequest{
		IncludeRegions:      includeRegions,
		ExcludeRegions:      excludeRegions,
		OnlyTravelers:       *onlyTravelers,
		OnlyLocalProvenance: *onlyLocalProvenance,
		State: &federation.FetchState{
			KeyCursor: &federation.Cursor{
				Timestamp: lastTime.Unix(),
				NextToken: *cursor,
			},
			RevisedKeyCursor: &federation.Cursor{
				Timestamp: lastRevisedTime.Unix(),
				NextToken: *revisedCursor,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build dial opts, optionally with security (recommended of course)
	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}
	if *skipAuth {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	} else {
		creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))

		idTokenSource, err := idtoken.NewTokenSource(ctx, *audience)
		if err != nil {
			log.Fatalf("Failed to create token source: %v", err)
		}
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(oauth.TokenSource{
			TokenSource: idTokenSource,
		}))
	}

	conn, err := grpc.Dial(*serverAddr, dialOpts...)
	if err != nil {
		log.Fatalf("Failed to dial %s: %v", *serverAddr, err)
	}
	defer conn.Close()

	total := 0
	response, err := federation.NewFederationClient(conn).Fetch(ctx, request)
	if err != nil {
		log.Fatalf("Error calling fetch: %v", err)
	}

	log.Printf("PRIMARY KEYS")
	for _, key := range response.Keys {
		log.Printf("p: %3d [bytes] interval: %d + %d traveler: %v regions: %v onset: %3d report: %v ",
			total, key.IntervalNumber, key.IntervalCount,
			key.Traveler, key.Regions, key.DaysSinceOnsetOfSymptoms, key.ReportType.String())
		total++
	}
	log.Printf("REVISED KEYS")
	for _, key := range response.RevisedKeys {
		log.Printf("r: %3d [bytes] interval: %d + %d traveler: %v regions: %v onset: %3d report: %v ",
			total, key.IntervalNumber, key.IntervalCount,
			key.Traveler, key.Regions, key.DaysSinceOnsetOfSymptoms, key.ReportType.String())
		total++
	}

	log.Printf("partialResponse: %t", response.PartialResponse)
	log.Printf("nextFetchState:  %+v", response.NextFetchState)
	log.Printf("number records:  %d", total)
}
