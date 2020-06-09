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

// Package flag includes custom flag parsing logic.
package flag

import (
	"fmt"
	"strings"
)

// RegionListVar is a list of upper-cased, unique regions derived from a comma-separated list.
type RegionListVar []string

func (l *RegionListVar) String() string {
	return fmt.Sprint(*l)
}

// Set parses the flag value into the final result.
func (l *RegionListVar) Set(val string) error {
	if len(*l) > 0 {
		return fmt.Errorf("already set")
	}

	unique := map[string]struct{}{}
	for _, v := range strings.Split(val, ",") {
		vf := strings.ToUpper(strings.TrimSpace(v))
		if _, seen := unique[vf]; !seen {
			*l = append(*l, vf)
			unique[vf] = struct{}{}
		}
	}
	return nil
}
