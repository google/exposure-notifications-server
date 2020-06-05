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

// This package is a CLI tool for creating federationin FederationQuery records.
package main

import (
	"context"
	"flag"
	"log"
	"regexp"
	"time"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/federationin/database"
	"github.com/google/exposure-notifications-server/internal/federationin/model"
	cflag "github.com/google/exposure-notifications-server/internal/flag"
	"github.com/google/exposure-notifications-server/internal/setup"
)

var (
	validQueryIDStr    = `\A[a-z][a-z0-9-_]*[a-z0-9]\z`
	validQueryIDRegexp = regexp.MustCompile(validQueryIDStr)

	validServerAddrStr    = `\A[a-z0-9.-]+(:\d+)?\z`
	validServerAddrRegexp = regexp.MustCompile(validServerAddrStr)

	queryID       = flag.String("query-id", "", "(Required) The ID of the federation query to set.")
	serverAddr    = flag.String("server-addr", "", "(Required) The address of the remote server, in the form some-server:some-port")
	audience      = flag.String("audience", federationin.DefaultAudience, "(Required) The OIDC audience to use when creating client tokens.")
	lastTimestamp = flag.String("last-timestamp", "", "The last timestamp (RFC3339) to set; queries start from this point and go forward.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	if *queryID == "" {
		log.Fatalf("--query-id is required")
	}
	if !validQueryIDRegexp.MatchString(*queryID) {
		log.Fatalf("--query-id %q must match %s", *queryID, validQueryIDStr)
	}
	if *serverAddr == "" {
		log.Fatalf("--server-addr is required")
	}
	if !validServerAddrRegexp.MatchString(*serverAddr) {
		log.Fatalf("--server-addr %q must match %s", *serverAddr, validServerAddrStr)
	}
	if *audience == "" {
		log.Fatalf("--audience is required")
	}
	if !federationin.ValidAudienceRegexp.MatchString(*audience) {
		log.Fatalf("--audience %q must match %s", *audience, federationin.ValidAudienceStr)
	}
	var lastTime time.Time
	if *lastTimestamp != "" {
		var err error
		lastTime, err = time.Parse(time.RFC3339, *lastTimestamp)
		if err != nil {
			log.Fatalf("failed to parse --last-timestamp (use RFC3339): %v", err)
		}
	}

	ctx := context.Background()
	var config coredb.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		log.Fatalf("failed to setup: %v", err)
	}
	defer env.Close(ctx)

	db := database.New(env.Database())

	query := &model.FederationInQuery{
		QueryID:        *queryID,
		ServerAddr:     *serverAddr,
		Audience:       *audience,
		IncludeRegions: includeRegions,
		ExcludeRegions: excludeRegions,
		LastTimestamp:  lastTime,
	}

	if err := db.AddFederationInQuery(ctx, query); err != nil {
		log.Fatalf("adding new query %s %#v: %v", *queryID, query, err)
	}

	log.Printf("Successfully added query %s %#v", *queryID, query)
}
