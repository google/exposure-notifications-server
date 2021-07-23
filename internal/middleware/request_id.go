// Copyright 2021 the Exposure Notifications Server authors
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

package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// contextKeyRequestID is the unique key in the context where the request ID is
// stored.
const contextKeyRequestID = contextKey("request_id")

// PopulateRequestID populates the request context with a random UUID if one
// does not already exist.
func PopulateRequestID() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if existing := RequestIDFromContext(ctx); existing == "" {
				u, err := uuid.NewRandom()
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				ctx = withRequestID(ctx, u.String())
				r = r.Clone(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDFromContext pulls the request ID from the context, if one was set.
// If one was not set, it returns the empty string.
func RequestIDFromContext(ctx context.Context) string {
	v := ctx.Value(contextKeyRequestID)
	if v == nil {
		return ""
	}

	t, ok := v.(string)
	if !ok {
		return ""
	}
	return t
}

// withRequestID sets the request ID on the provided context, returning a new
// context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}
