// This package is a CLI tool for setting federation queries.
package main

import (
	"cambio/pkg/database"
	cflag "cambio/pkg/flag"
	"cambio/pkg/model"
	"context"
	"flag"
	"log"
	"regexp"
	"time"
)

var (
	validQueryIDStr    = `\A[a-z][a-z0-9-_]*[a-z0-9]\z`
	validQueryIDRegexp = regexp.MustCompile(validQueryIDStr)

	validServerAddrStr    = `\A[a-z0-9.-]+(:\d+)?\z`
	validServerAddrRegexp = regexp.MustCompile(validServerAddrStr)

	queryID       = flag.String("query-id", "", "(Required) The ID of the federation query to set.")
	serverAddr    = flag.String("server-addr", "", "(Required) The address of the remote server, in the form some-server:some-port")
	lastTimestamp = flag.String("last-timestamp", "", "The last timestamp (RFC3339) to set; queries start from this point and go forward.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	if *queryID == "" {
		log.Fatalf("query-id is required")
	}
	if !validQueryIDRegexp.MatchString(*queryID) {
		log.Fatalf("query-id %q must match %s", *queryID, validQueryIDStr)
	}
	if *serverAddr == "" {
		log.Fatalf("server-addr is required")
	}
	if !validServerAddrRegexp.MatchString(*serverAddr) {
		log.Fatalf("server-addr %q must match %s", *serverAddr, validServerAddrStr)
	}

	if err := database.Initialize(); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	var lastTime time.Time
	if *lastTimestamp != "" {
		var err error
		lastTime, err = time.Parse(time.RFC3339, *lastTimestamp)
		if err != nil {
			log.Fatalf("failed to parse --last-timestamp (use RFC3339): %v", err)
		}
	}

	query := &model.FederationQuery{
		ServerAddr:     *serverAddr,
		IncludeRegions: includeRegions,
		ExcludeRegions: excludeRegions,
		LastTimestamp:  lastTime,
	}

	ctx := context.Background()
	if err := database.AddFederationQuery(ctx, *queryID, query); err != nil {
		log.Fatalf("adding new query %s %#v: %v", *queryID, query, err)
	}

	log.Printf("Successfully added query %s %#v", *queryID, query)
}
