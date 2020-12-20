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

package publish

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/publish/model"
)

func TestPlatform(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		userAgent    string
		wantPlatform string
	}{
		{
			name:         "iOS_with_iphone",
			userAgent:    "some test IPHONE some other text",
			wantPlatform: model.PlatformIOS,
		},
		{
			name:         "ios_bluetoothd",
			userAgent:    "bluetoothd (unknown version) CFNetwork/1206 Darwin/20.1.0",
			wantPlatform: model.PlatformIOS,
		},
		{
			name:         "android_nokia",
			userAgent:    "Dalvik/2.1.0 (Linux; U; Android 9; Nokia 3.1 C Build/PPR1.180610.011)",
			wantPlatform: model.PlatformAndroid,
		},
		{
			name:         "android_pixel",
			userAgent:    "Dalvik/2.1.0 (Linux; U; Android 10; Pixel 4 Build/QQ3A.200805.001)",
			wantPlatform: model.PlatformAndroid,
		},
		{
			name:         "unknown",
			userAgent:    "Google Chrome",
			wantPlatform: model.PlatformUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(""))
			r.Header.Set("User-Agent", tc.userAgent)

			if got := platform(r); got != tc.wantPlatform {
				t.Fatalf("wrong platform, want: %q got: %q", tc.wantPlatform, got)
			}
		})
	}

}
