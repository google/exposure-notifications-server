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
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/internal/publish/model"
)

func platform(r *http.Request) string {
	userAgent := r.UserAgent()
	switch {
	case isAndroid(userAgent):
		return model.PlatformAndroid
	case isIOS(userAgent):
		return model.PlatformIOS
	default:
		return model.PlatformUnknown
	}
}

// isAndroid determines if a User-Agent is a Android device.
func isAndroid(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "android")
}

// isIOS determines if a User-Agent is an iOS EN device.
// for EN Express publishes, the useragent is "bluetoothd" and "cfnetwork"
func isIOS(userAgent string) bool {
	lowerAgent := strings.ToLower(userAgent)
	return strings.Contains(lowerAgent, "iphone") ||
		(strings.Contains(lowerAgent, "bluetoothd") && strings.Contains(lowerAgent, "cfnetwork"))
}
