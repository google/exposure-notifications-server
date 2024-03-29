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

package v1

import (
	"testing"

	"github.com/google/exposure-notifications-server/pkg/errcmp"
)

func TestValidateClaims(t *testing.T) {
	t.Parallel()

	c := NewVerificationClaims()
	c.ReportType = "bogus"

	err := c.CustomClaimsValid()
	errcmp.MustMatch(t, err, "bogus")

	for k := range ValidReportTypes {
		c.ReportType = k
		if err := c.CustomClaimsValid(); err != nil {
			t.Errorf("got error when using valid report type: %q, err: %v", k, err)
		}
	}
}
