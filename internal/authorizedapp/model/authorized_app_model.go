// Copyright 2020 the Exposure Notifications Server authors
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

// Package model is a model abstraction of authorized apps.
package model

import (
	"sort"
	"strings"
)

// AuthorizedApp represents the configuration for a single exposure notification
// application and their access to and requirements for using the API. DB times
// of 0 are interpreted to be "unbounded" in that direction.
type AuthorizedApp struct {
	// AppPackageName is the name of the package like com.company.app.
	AppPackageName string

	// AllowedRegions is the list of allowed regions for this app. If the list is
	// empty, all regions are permitted.
	AllowedRegions map[string]struct{}

	// AllowedHealthAuthorityIDs represents the set of allowed health authorities
	// that this app can obtain and verify diagnosis verification certificates from.
	AllowedHealthAuthorityIDs         map[int64]struct{}
	BypassHealthAuthorityVerification bool

	// If true - revision tokens will still be accepted and checked, but will not
	// enforce correctness. They will still be generated as output.
	BypassRevisionToken bool
}

// NewAuthorizedApp initializes an AuthorizedApp structure including
// pre-allocating all included maps.
func NewAuthorizedApp() *AuthorizedApp {
	return &AuthorizedApp{
		AllowedRegions:            make(map[string]struct{}),
		AllowedHealthAuthorityIDs: make(map[int64]struct{}),
	}
}

// AllAllowedRegions returns a slice of all allowed region codes.
func (c *AuthorizedApp) AllAllowedRegions() []string {
	regions := []string{}
	for k := range c.AllowedRegions {
		regions = append(regions, k)
	}
	return regions
}

// AllAllowedHealthAuthorityIDs returns a slice of all allowed
// heauth authority IDs.
func (c *AuthorizedApp) AllAllowedHealthAuthorityIDs() []int64 {
	has := []int64{}
	for k := range c.AllowedHealthAuthorityIDs {
		has = append(has, k)
	}
	return has
}

// Validate checks an authorized app before a save operation.
func (c *AuthorizedApp) Validate() []string {
	errors := make([]string, 0)
	if c.AppPackageName == "" {
		errors = append(errors, "Health Authority ID cannot be empty")
	}
	if len(c.AllowedRegions) == 0 {
		errors = append(errors, "Regions list cannot be empty")
	}
	return errors
}

// RegionsOnePerLine returns a string with all authorized
// regions, one per line. This is a utility method for the
// admin console.
func (c *AuthorizedApp) RegionsOnePerLine() string {
	var regions sort.StringSlice
	for r := range c.AllowedRegions {
		regions = append(regions, r)
	}
	regions.Sort()
	return strings.Join(regions, "\n")
}

// IsAllowedRegion returns true if the regions list is empty or if the given
// region is in the list of allowed regions.
func (c *AuthorizedApp) IsAllowedRegion(s string) bool {
	// This is a legacy inconsistency. Anything going through the admin
	// console will have allowed regions set.
	if len(c.AllowedRegions) == 0 {
		return true
	}

	_, ok := c.AllowedRegions[s]
	return ok
}
