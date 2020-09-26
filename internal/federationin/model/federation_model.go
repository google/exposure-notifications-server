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

// Package model is a model abstraction of federation in.
package model

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/pb/federation"
)

// FederationInQuery represents a configuration to pull federation results from other servers.
type FederationInQuery struct {
	QueryID             string
	ServerAddr          string
	Audience            string
	IncludeRegions      []string
	ExcludeRegions      []string
	OnlyLocalProvenance bool
	OnlyTravelers       bool

	// FetchState items.
	LastTimestamp        time.Time
	LastCursor           string
	LastRevisedTimestamp time.Time
	LastRevisedCursor    string
}

// UpdateFetchState updates the query state based on the fetch state returned from a federation pull.
func (q *FederationInQuery) UpdateFetchState(fs *federation.FetchState) {
	if fs.KeyCursor == nil {
		q.LastTimestamp = time.Time{}
		q.LastCursor = ""
	} else {
		q.LastTimestamp = time.Unix(fs.KeyCursor.Timestamp, 0).UTC()
		q.LastCursor = fs.KeyCursor.NextToken
	}

	if fs.RevisedKeyCursor == nil {
		q.LastRevisedTimestamp = time.Time{}
		q.LastRevisedCursor = ""
	} else {
		q.LastRevisedTimestamp = time.Unix(fs.RevisedKeyCursor.Timestamp, 0).UTC()
		q.LastRevisedCursor = fs.RevisedKeyCursor.NextToken
	}
}

// FetchState returns the federation fetch state based on query state.
func (q *FederationInQuery) FetchState() *federation.FetchState {
	return &federation.FetchState{
		KeyCursor: &federation.Cursor{
			Timestamp: q.LastTimestamp.Unix(),
			NextToken: q.LastCursor,
		},
		RevisedKeyCursor: &federation.Cursor{
			Timestamp: q.LastRevisedTimestamp.Unix(),
			NextToken: q.LastRevisedCursor,
		},
	}
}

// FederationInSync is the result of a federation query pulled from other servers.
type FederationInSync struct {
	SyncID              int64
	QueryID             string
	Started             time.Time
	Completed           time.Time
	Insertions          int
	MaxTimestamp        time.Time
	MaxRevisedTimestamp time.Time
}

// FederationOutAuthorization is an authorized client that reads federation data from this server.
type FederationOutAuthorization struct {
	Issuer  string
	Subject string
	// Audience is optional, but will be validated against the OIDC token if provided.
	Audience       string
	Note           string
	IncludeRegions []string
	ExcludeRegions []string
}
