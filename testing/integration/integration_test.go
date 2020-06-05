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

package integration

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
)

// This tests that the integration util can bring up the Monolith.
// It should eventually be swapped out for a smoke test that ensures
// the server is listening at all the relevant addresses.
func TestIntegrationUtilSmoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	StartSystemUnderTest(t, ctx)
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		t.Errorf("Health check generated error: %v", err)
	}
	ok, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("Couldn't read health check response: %v", err)
	}
	if string(ok) != "OK" {
		t.Errorf("Health check did not generate response \"OK\". Actual: %v", ok)
	}
}
