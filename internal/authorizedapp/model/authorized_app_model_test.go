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

package model

import (
	"testing"
)

func TestAuthorizedApp_IsAllowedRegion(t *testing.T) {
	cfg := NewAuthorizedApp()
	cfg.AllowedRegions = map[string]struct{}{
		"US": {},
	}

	if ok := cfg.IsAllowedRegion("US"); !ok {
		t.Errorf("expected US to be allowed")
	}

	if ok := cfg.IsAllowedRegion("CA"); ok {
		t.Errorf("expected CA to not be allowed")
	}
}
