// Copyright 2021 Google LLC
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

import "testing"

func TestTotal(t *testing.T) {
	p := &PublishRequests{
		UnknownPlatform: 6,
		Android:         22,
		IOS:             45,
	}
	if want, got := int64(73), p.Total(); want != got {
		t.Fatalf("addition not working, want: %v got: %v", want, got)
	}
}
