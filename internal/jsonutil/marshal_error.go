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

package jsonutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func MarshalResponse(w http.ResponseWriter, status int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := escapeJSON(err.Error())
		fmt.Fprintf(w, jsonErrTmpl, msg)
		return
	}

	w.WriteHeader(status)
	fmt.Fprintf(w, "%s", data)
}

// escapeJSON does primitive JSON escaping.
func escapeJSON(s string) string {
	return strings.Replace(s, `"`, `\"`, -1)
}

// jsonErrTmpl is the template to use when returning a JSON error. It is
// rendered using Printf, not json.Encode, so values must be escaped by the
// caller.
const jsonErrTmpl = `{"error":"%s"}`
