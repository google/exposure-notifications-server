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

// Package jsonutil provides common utilities for properly handling JSON payloads in HTTP body.
package jsonutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Max request size of 64KB. None of the current API requests for this
	// server are near that limit. Prevents us from unnecessarily parsing JSON
	// payloads that are much large than we anticipate.
	maxBodyBytes = 64_000
)

// Unmarshal provides a common implementation of JSON unmarshalling with well defined error handling.
func Unmarshal(w http.ResponseWriter, r *http.Request, data interface{}) (int, error) {
	if t := r.Header.Get("Content-type"); t != "application/json" {
		return http.StatusUnsupportedMediaType, fmt.Errorf("content-type is not application/json")
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(&data); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return http.StatusBadRequest, fmt.Errorf("malformed json at position %v", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return http.StatusBadRequest, fmt.Errorf("malformed json")
		case errors.As(err, &unmarshalError):
			return http.StatusBadRequest, fmt.Errorf("invalid value %v at position %v", unmarshalError.Field, unmarshalError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return http.StatusBadRequest, fmt.Errorf("unknown field %s", fieldName)
		case errors.Is(err, io.EOF):
			return http.StatusBadRequest, fmt.Errorf("body must not be empty")
		case err.Error() == "http: request body too large":
			return http.StatusRequestEntityTooLarge, err
		default:
			return http.StatusInternalServerError, fmt.Errorf("failed to decode json %v", err)
		}
	}
	if d.More() {
		return http.StatusBadRequest, fmt.Errorf("body must contain only one JSON object")
	}

	return http.StatusOK, nil
}
