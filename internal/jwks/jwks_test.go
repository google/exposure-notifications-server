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

package jwks

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/project"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

var key1 = `{"kid":"r2v1","kty":"EC","crv":"P-256","x":"qcMMcLX1Z2afVAzypTMw1g3KN_OcdgvRDwOgpDWiswU","y":"RjK8Hc7pLLO_JADNhwZIxCXjCH95VHuWPoKVaCGkXiA"}`
var key2 = `{"kid":"r2v2","kty":"EC","crv":"P-256","x":"F2MgtKg_cm-JfcJlrEUJMgXqXq1vHWRPMbBWEjzmN0U","y":"4U6g0nX9mVOGaHL8kYX10gL4Fsj-wNb4V9GMSJ7iLKk"}`
var enc1 = `MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEqcMMcLX1Z2afVAzypTMw1g3KN/Oc
dgvRDwOgpDWiswVGMrwdzukss78kAM2HBkjEJeMIf3lUe5Y+gpVoIaReIA==`
var enc2 = `MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEF2MgtKg/cm+JfcJlrEUJMgXqXq1v
HWRPMbBWEjzmN0XhTqDSdf2ZU4ZocvyRhfXSAvgWyP7A1vhX0YxInuIsqQ==`

func encodeKeys(keys ...string) string {
	return "[" + strings.Join(keys, ",") + "]"
}

func encodeKey(key string) string {
	return "-----BEGIN PUBLIC KEY-----\n" + key + "\n-----END PUBLIC KEY-----"
}

func encodePublic(keys ...string) []string {
	ret := make([]string, len(keys))
	for i := range keys {
		ret[i] = encodeKey(keys[i])
	}
	return ret
}

// This test is fairly exhaustive -- testing all the pieces of the JWKS
// endpoint, and then an "end-to-end" test of making sure the DB is updated
// as well. It might behoove future engineers to break this up into multiple
// tests.
func TestUpdateHA(t *testing.T) {
	t.Parallel()
	//
	// Constants for testing.
	//

	// HealthAuthorities
	haEmpty := model.HealthAuthority{}
	ha1 := model.HealthAuthority{
		Keys: []*model.HealthAuthorityKey{
			{Version: "r2v1", PublicKeyPEM: encodePublic(enc1)[0]},
		},
	}
	ha2 := model.HealthAuthority{
		Keys: []*model.HealthAuthorityKey{
			{Version: "r2v2", PublicKeyPEM: encodePublic(enc2)[0]},
		},
	}
	ha2Keys := model.HealthAuthority{
		Keys: []*model.HealthAuthorityKey{
			{Version: "r2v1", PublicKeyPEM: encodePublic(enc1)[0]},
			{Version: "r2v2", PublicKeyPEM: encodePublic(enc2)[0]},
		},
	}

	// version maps
	v1 := map[string]string{encodePublic(enc1)[0]: "r2v1"}
	v2 := map[string]string{encodePublic(enc2)[0]: "r2v2"}
	v12 := map[string]string{encodePublic(enc1)[0]: "r2v1", encodePublic(enc2)[0]: "r2v2"}

	tests := []struct {
		name     string
		resp     string
		encoded  []string
		versions map[string]string
		ha       model.HealthAuthority
		deadKeys []int
		newKeys  []string
		resKeys  []string
	}{
		// no update
		{"nop key1", encodeKeys(key1), encodePublic(enc1), v1, ha1, nil, nil, encodePublic(enc1)},
		{"nop key1", encodeKeys(key2), encodePublic(enc2), v2, ha2, nil, nil, encodePublic(enc2)},

		// new keys
		{"new key0-1", encodeKeys(key1), encodePublic(enc1), v1, haEmpty, nil, encodePublic(enc1), encodePublic(enc1)},
		{"new key0-2", encodeKeys(key2), encodePublic(enc2), v2, haEmpty, nil, encodePublic(enc2), encodePublic(enc2)},
		{"new key1-2", encodeKeys(key1, key2), encodePublic(enc1, enc2), v12, ha1, nil, encodePublic(enc2), encodePublic(enc1, enc2)},
		{"new key0-12", encodeKeys(key1, key2), encodePublic(enc1, enc2), v12, haEmpty, nil, encodePublic(enc1, enc2), encodePublic(enc1, enc2)},
		{"new key1-2/del", encodeKeys(key1), encodePublic(enc1), v1, ha2, []int{0}, encodePublic(enc1), encodePublic(enc1, enc2)},

		// delete keys
		{"del2", "", nil, nil, ha2Keys, []int{0, 1}, nil, encodePublic(enc1, enc2)},
	}

	// Run the tests.
	//
	// Again, each test tests individual pieces of the service, and then an
	// "end-to-end" test is run where the DB is validated.
	for i, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Start a local server to serve JSON data for the test.
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, test.resp)
				w.Header().Set("Content-Type", "application/json")
			}))
			defer ts.Close()

			// Set up the test.
			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)
			mgr, err := NewManager(testDB, time.Minute, 5*time.Second)
			if err != nil {
				t.Fatalf("[%d] unexpected error: %v", i, err)
			}
			jwksURI := ts.URL
			ha := &model.HealthAuthority{JwksURI: &jwksURI}

			// Test networking.
			rxKeys, err := mgr.getKeys(ctx, ha)
			if err != nil {
				t.Fatalf("[%d] unexpected error: %v", i, err)
			}
			if string(rxKeys) != test.resp {
				t.Fatalf("[%d] expected %v, got %v", i, test.resp, rxKeys)
			}

			// Test the encoding of the keys/versions. (basically, can we decode the
			// JWKS strings.)
			var encoded []string
			var versions map[string]string
			encoded, versions, err = parseKeys(rxKeys)
			if err != nil {
				t.Fatalf("[%d] unexpected error: %v", i, err)
			}
			if !reflect.DeepEqual(encoded, test.encoded) {
				t.Fatalf("[%d] encoded strings aren't equal expected %v, got %v", i, test.encoded, encoded)
			}
			if !reflect.DeepEqual(versions, test.versions) {
				t.Fatalf("[%d] versions aren't equal expected %v, got %v", i, test.versions, versions)
			}

			// Test we have correctly identified if keys are deleted or new, etc.
			deadKeys, newKeys := findKeyMods(&test.ha, encoded)
			if !reflect.DeepEqual(deadKeys, test.deadKeys) {
				t.Fatalf("[%d] dead keys aren't equal expected %v, got %v", i, test.deadKeys, deadKeys)
			}
			if !reflect.DeepEqual(newKeys, test.newKeys) {
				t.Fatalf("[%d] new keys aren't equal expected %v, got %v", i, test.newKeys, newKeys)
			}

			//
			// Now test end-to-end.
			//
			test.ha.JwksURI = &jwksURI

			// Add the HealthAuthority & Keys to the DB. Note, we need to remove all
			// keys from the testing HealthAuthority before adding it to the DB as it's
			// checked for empty. Also the function we're calling below (updateHA)
			// expects the keys to be empty in the HealthAuthority.
			var keys []*model.HealthAuthorityKey
			keys, test.ha.Keys = test.ha.Keys, nil
			haDB := hadb.New(mgr.db)
			test.ha.Issuer, test.ha.Audience, test.ha.Name = "ISSUER", "AUDIENCE", "NAME"
			if err := haDB.AddHealthAuthority(ctx, &test.ha); err != nil {
				t.Fatalf("[%d] error adding the HealthAuthority, %v", i, err)
			}
			for _, key := range keys {
				if err := haDB.AddHealthAuthorityKey(ctx, &test.ha, key); err != nil {
					t.Errorf("[%d] error adding key: %v", i, err)
				}
			}

			// Now, run the whole flow for a HealthAuthority.
			if err := mgr.updateHA(ctx, &test.ha); err != nil {
				t.Fatalf("[%d] error updating: %v", i, err)
			}

			// Check the DB.
			if ha, err := haDB.GetHealthAuthorityByID(ctx, test.ha.ID); err != nil {
				t.Fatalf("[%d] error retreiving HealthAuthority: %v", i, err)
			} else {
				// Check the resultant keys.
				if len(ha.Keys) != len(test.resKeys) {
					t.Fatalf("[%d] key lengths different %d, expected %d", i, len(ha.Keys), len(test.resKeys))
				}
				for j := range ha.Keys {
					if key := ha.Keys[j].PublicKeyPEM; key != test.resKeys[j] {
						t.Errorf("[%d] wrong key[%d] %q, expected %q", i, j, key, test.resKeys[j])
					}
				}

				// Check to see if we've deleted keys as needed as well, (ie the From time is
				// populated).
				var emptyTime time.Time
				for _, j := range test.deadKeys {
					if delTime := ha.Keys[j].From; reflect.DeepEqual(emptyTime, delTime) {
						t.Errorf("[%d:%d] key not deleted %v", i, j, delTime)
					}
				}
			}
		})
	}
}

func TestStrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		k1, k2 string
	}{
		{"foo", "foo"},
		{"foo", "\nfoo\n"},
		{enc1, enc1},
		{encodeKey(enc1), enc1},
	}
	for i, test := range tests {
		k2 := strings.ReplaceAll(test.k2, "\n", "")
		if stripped := stripKey(test.k1); stripped != k2 {
			t.Errorf("[%d] compareKeys(%q) = %q, want %q", i, test.k1, stripped, k2)
		}
	}
}
