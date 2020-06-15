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

// Package base64util extracts base64 encoding/decoding logic into a single API
// that is tolerant of various paddings.
//
// Although exported, this package is non intended for general consumption.
// It is a shared dependency between multiple exposure notifications projects.
// We cannot guarantee that there won't be breaking changes in the future.
package base64util

import (
	"encoding/base64"
	"strings"
)

// DecodeString decodes the given string as base64.
func DecodeString(s string) ([]byte, error) {
	s = unpad(convertToURLEncoding(s))
	return base64.RawURLEncoding.DecodeString(s)
}

// convertToURLEncoding converts the given string to URL-encoded.
func convertToURLEncoding(s string) string {
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}

// unpad removes any padding from the string.
func unpad(s string) string {
	return strings.TrimRight(s, "=")
}
