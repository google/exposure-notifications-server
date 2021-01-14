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
	"time"
)

func TestShouldTry(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	retryRate := time.Hour

	tests := []struct {
		f  *ImportFile
		ex bool
	}{
		{&ImportFile{Status: ImportFileOpen}, true},
		{&ImportFile{Status: ImportFilePending}, false},
		{&ImportFile{Status: ImportFileComplete}, false},
		{&ImportFile{Status: ImportFileFailed}, false},
		{&ImportFile{Status: ImportFileOpen, Retries: 1, DiscoveredAt: now.Add(-time.Minute)}, false},
		{&ImportFile{Status: ImportFileOpen, Retries: 1, DiscoveredAt: now.Add(-(retryRate + time.Minute))}, true},
	}
	for i, test := range tests {
		if test.f.ShouldTry(retryRate) != test.ex {
			t.Errorf("[%d] %v.ShouldTry() = %v, expected %v", i, test.f, !test.ex, test.ex)
		}
	}
}
