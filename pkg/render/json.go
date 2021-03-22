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

package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-multierror"
)

// RenderJSON renders the interface as JSON. It attempts to gracefully handle
// any rendering errors to avoid partial responses sent to the response by
// writing to a buffer first, then flushing the buffer to the response.
//
// If the provided data is nil and the response code is a 200, the result will
// be `{"ok":true}`. If the code is not a 200, the response will be of the
// format `{"error":"<val>"}` where val is the JSON-escaped http.StatusText for
// the provided code.
//
// If rendering fails, a generic 500 JSON response is returned. In dev mode, the
// error is included in the payload. If flushing the buffer to the response
// fails, an error is logged, but no recovery is attempted.
//
// The buffers are fetched via a sync.Pool to reduce allocations and improve
// performance.
func (r *Renderer) RenderJSON(w http.ResponseWriter, code int, data interface{}) {
	// Avoid marshaling nil data.
	if data == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		// Return an OK response.
		if code >= 200 && code < 300 {
			fmt.Fprint(w, jsonOKResp)
			return
		}

		fmt.Fprintf(w, jsonErrTmpl, http.StatusText(code))
		return
	}

	// Special-case handle multi-error.
	if typ, ok := data.(*multierror.Error); ok {
		errs := typ.WrappedErrors()
		msgs := make([]string, 0, len(errs))
		for _, err := range errs {
			msgs = append(msgs, err.Error())
		}
		data = &multiError{Errors: msgs}
	}

	// If the provided value was an error, marshall accordingly.
	if typ, ok := data.(error); ok {
		data = &singleError{Error: typ.Error()}
	}

	// Acquire a renderer
	b := r.pool.Get().(*bytes.Buffer)
	b.Reset()
	defer r.pool.Put(b)

	// Render into the renderer
	if err := json.NewEncoder(b).Encode(data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, jsonErrTmpl, http.StatusText(http.StatusInternalServerError))
		return
	}

	// Rendering worked, flush to the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = b.WriteTo(w)
}

// jsonErrTmpl is the template to use when returning a JSON error. It is
// rendered using Printf, not json.Encode, so values must be escaped by the
// caller.
const jsonErrTmpl = `{"error":"%s"}`

// jsonOKResp is the return value for empty data responses.
const jsonOKResp = `{"ok":true}`

type singleError struct {
	Error string `json:"error,omitempty"`
}

type multiError struct {
	Errors []string `json:"errors,omitempty"`
}
