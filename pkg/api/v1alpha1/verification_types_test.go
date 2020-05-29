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

package v1alpha1

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTransmissionRiskVectorSort(t *testing.T) {

	got := TransmissionRiskVector{
		{0, 0},
		{3, 100},
		{5, 200},
	}
	sort.Sort(got)

	want := TransmissionRiskVector{{5, 200}, {3, 100}, {0, 0}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("sort(TransmissionRiskVector) mismatch (-want +got):\n%v", diff)
	}
}
