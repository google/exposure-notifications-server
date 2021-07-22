// Copyright 2021 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"io"
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
)

func TestHandleHealthz(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	_, s := newTestServer(t)
	server := newHTTPServer(t, http.MethodGet, "/", s.HandleHealthz())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error making http call: %v", err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, 200; got != want {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Errorf("expected status %d to be %d; headers: %#v; body: %s", got, want, resp.Header, b)
	}

	mustFindStrings(t, resp, "ok")
}
