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

// Package cleanup implements the API handlers for running data deletion jobs.
package cleanup

import (
	"fmt"
	"time"
)

const minTTL = 10 * 24 * time.Hour

func cutoffDate(d time.Duration, override bool) (time.Time, error) {
	if d >= minTTL || override {
		return time.Now().UTC().Add(-d), nil
	}

	return time.Time{}, fmt.Errorf("cleanup ttl %s is less than configured minimum ttl of %s", d, minTTL)
}
