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

// This package is a CLI tool for creating FederationAuthorization entries, controlling who can access the federationout endpoint.
// If the subject/issuer match an existing record, that record will be updated.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/google/exposure-notifications-server/internal/api/federationin"
	"github.com/google/exposure-notifications-server/internal/database"
	cflag "github.com/google/exposure-notifications-server/internal/flag"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/kelseyhightower/envconfig"
)

const (
	defaultIssuer = "https://accounts.google.com"
)

var (
	testRegions = []string{"TEST", "PROBE"}

	subject  = flag.String("subject", "", "(Required) The OIDC subject (for issuer https://accounts.google.com, this is the obfuscated Gaia ID.)")
	audience = flag.String("audience", federationin.DefaultAudience, "The OIDC audience; leaving this blank will cause server to not enforce the audience claim.")
	note     = flag.String("note", "", "An open text note to include on the record.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	if *subject == "" {
		log.Fatalf("--subject is required")
	}

	// Issue warnings about missing test regions in excludeRegions.
	var missingTestRegions []string
	for _, testRegion := range testRegions {
		inExcluded := false
		for _, excludedRegion := range excludeRegions {
			if excludedRegion == testRegion {
				inExcluded = true
				break
			}
		}
		if !inExcluded {
			missingTestRegions = append(missingTestRegions, testRegion)
		}
	}
	if len(missingTestRegions) > 0 {
		log.Printf("\n\nWARNING: This record does not exclude test regions %q and is only appropriate for a test federation authorization.\n\n", missingTestRegions)
	}

	ctx := context.Background()
	var config database.Config
	err := envconfig.Process("database", &config)
	if err != nil {
		log.Fatalf("error loading environment variables: %v", err)
	}

	db, err := database.NewFromEnv(ctx, &config)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	auth := &model.FederationAuthorization{
		Issuer:         defaultIssuer, // Authorization interceptor currently only supports defaultIssuer.
		Subject:        *subject,
		Audience:       *audience,
		Note:           *note,
		IncludeRegions: includeRegions,
		ExcludeRegions: excludeRegions,
	}

	if err := db.AddFederationAuthorization(ctx, auth); err != nil {
		log.Fatalf("adding new federation client authorization %#v: %v", auth, err)
	}

	log.Printf("Successfully added federation client authorization %#v", auth)
}
