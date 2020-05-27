// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import "time"

// CombineDateAndTime takes values from date and time HTML inputs and combines
// them to a single date time.
func CombineDateAndTime(dateS, timeS string) (time.Time, error) {
	if dateS == "" {
		// Return zero time.
		return time.Time{}, nil
	}
	if timeS == "" {
		timeS = "00:00"
	}
	return time.Parse("2006-01-02 15:04", dateS+" "+timeS)
}
