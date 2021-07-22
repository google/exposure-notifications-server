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

package database

import (
	"testing"
	"time"
)

var testDatabaseInstance *TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestNullableTime(t *testing.T) {
	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		t.Parallel()

		if got, want := NullableTime(time.Time{}), (*time.Time)(nil); got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})

	t.Run("not_nil", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		if got, want := NullableTime(now), &now; !got.Equal(now) {
			t.Errorf("expected %q to be %q", got, want)
		}
	})
}
