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

// Package buildinfo provides high-level build information injected during
// build.
package buildinfo

var (
	// id is the unique build identifier.
	id string = "unknown"

	// tag is the git tag from which this build was created.
	tag string = "unknown"
)

// info provides the build information about the key server.
type buildinfo struct{}

// ID returns the build ID.
func (buildinfo) ID() string {
	return id
}

// Tag returns the build tag.
func (buildinfo) Tag() string {
	return tag
}

// KeyServer provides the build information about the key server.
var KeyServer buildinfo
