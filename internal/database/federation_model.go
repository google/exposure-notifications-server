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

package database

import (
	"time"
)

// FederationInQuery represents a configuration to pull federation results from other servers.
type FederationInQuery struct {
	QueryID        string    `db:"query_id"`
	ServerAddr     string    `db:"server_addr"`
	Audience       string    `db:"oidc_audience"`
	IncludeRegions []string  `db:"include_regions"`
	ExcludeRegions []string  `db:"exclude_regions"`
	LastTimestamp  time.Time `db:"last_timestamp"`
}

// FederationInSync is the result of a federation query pulled from other servers.
type FederationInSync struct {
	SyncID       int64     `db:"sync_id"`
	QueryID      string    `db:"query_id"`
	Started      time.Time `db:"started"`
	Completed    time.Time `db:"completed"`
	Insertions   int       `db:"insertions"`
	MaxTimestamp time.Time `db:"max_timestamp"`
}

// FederationOutAuthorization is an authorized client that reads federation data from this server.
type FederationOutAuthorization struct {
	Issuer  string `db:"oidc_issuer"`
	Subject string `db:"oidc_subject"`
	// Audience is optional, but will be validated against the OIDC token if provided.
	Audience       string   `db:"oidc_audience"`
	Note           string   `db:"note"`
	IncludeRegions []string `db:"include_regions"`
	ExcludeRegions []string `db:"exclude_regions"`
}
