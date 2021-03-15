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

	"github.com/google/exposure-notifications-server/internal/project"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/errcmp"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

var (
	key1 = `{"kid":"r2v1","kty":"EC","crv":"P-256","x":"qcMMcLX1Z2afVAzypTMw1g3KN_OcdgvRDwOgpDWiswU","y":"RjK8Hc7pLLO_JADNhwZIxCXjCH95VHuWPoKVaCGkXiA"}`
	key2 = `{"kid":"r2v2","kty":"EC","crv":"P-256","x":"F2MgtKg_cm-JfcJlrEUJMgXqXq1vHWRPMbBWEjzmN0U","y":"4U6g0nX9mVOGaHL8kYX10gL4Fsj-wNb4V9GMSJ7iLKk"}`
	enc1 = `MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEqcMMcLX1Z2afVAzypTMw1g3KN/Oc
dgvRDwOgpDWiswVGMrwdzukss78kAM2HBkjEJeMIf3lUe5Y+gpVoIaReIA==`
)

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
	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Start a local server to serve JSON data for the test.
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, tc.resp)
				w.Header().Set("Content-Type", "application/json")
			}))
			defer ts.Close()

			// Set up the tc.
			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)
			mgr, err := NewManager(testDB, time.Minute, 5*time.Second, 2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			jwksURI := ts.URL
			ha := &model.HealthAuthority{JwksURI: &jwksURI}

			// Test networking.
			rxKeys, err := mgr.getKeys(ctx, ha)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(rxKeys) != tc.resp {
				t.Fatalf("expected %v, got %v", tc.resp, rxKeys)
			}

			// Test the encoding of the keys/versions. (basically, can we decode the
			// JWKS strings.)
			var encoded []string
			var versions map[string]string
			encoded, versions, err = parseKeys(rxKeys)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(encoded, tc.encoded) {
				t.Fatalf("encoded strings aren't equal expected %v, got %v", tc.encoded, encoded)
			}
			if !reflect.DeepEqual(versions, tc.versions) {
				t.Fatalf("versions aren't equal expected %v, got %v", tc.versions, versions)
			}

			// Test we have correctly identified if keys are deleted or new, etc.
			deadKeys, newKeys := findKeyMods(&tc.ha, encoded)
			if !reflect.DeepEqual(deadKeys, tc.deadKeys) {
				t.Fatalf("dead keys aren't equal expected %v, got %v", tc.deadKeys, deadKeys)
			}
			if !reflect.DeepEqual(newKeys, tc.newKeys) {
				t.Fatalf("new keys aren't equal expected %v, got %v", tc.newKeys, newKeys)
			}

			//
			// Now test end-to-end.
			//
			tc.ha.JwksURI = &jwksURI

			// Add the HealthAuthority & Keys to the DB. Note, we need to remove all
			// keys from the testing HealthAuthority before adding it to the DB as it's
			// checked for empty. Also the function we're calling below (updateHA)
			// expects the keys to be empty in the HealthAuthority.
			var keys []*model.HealthAuthorityKey
			keys, tc.ha.Keys = tc.ha.Keys, nil
			haDB := hadb.New(mgr.db)
			tc.ha.Issuer, tc.ha.Audience, tc.ha.Name = "ISSUER", "AUDIENCE", "NAME"
			if err := haDB.AddHealthAuthority(ctx, &tc.ha); err != nil {
				t.Fatalf("error adding the HealthAuthority, %v", err)
			}
			for _, key := range keys {
				if err := haDB.AddHealthAuthorityKey(ctx, &tc.ha, key); err != nil {
					t.Errorf("error adding key: %v", err)
				}
			}

			// Now, run the whole flow for a HealthAuthority.
			if err := mgr.updateHA(ctx, &tc.ha); err != nil {
				t.Fatalf("error updating: %v", err)
			}

			// Check the DB.
			if ha, err := haDB.GetHealthAuthorityByID(ctx, tc.ha.ID); err != nil {
				t.Fatalf("error retreiving HealthAuthority: %v", err)
			} else {
				// Check the resultant keys.
				if len(ha.Keys) != len(tc.resKeys) {
					t.Fatalf("key lengths different %d, expected %d", len(ha.Keys), len(tc.resKeys))
				}
				for j := range ha.Keys {
					if key := ha.Keys[j].PublicKeyPEM; key != tc.resKeys[j] {
						t.Errorf("wrong key %q, expected %q", key, tc.resKeys[j])
					}
				}

				// Check to see if we've deleted keys as needed as well, (ie the From time is
				// populated).
				var emptyTime time.Time
				for _, j := range tc.deadKeys {
					if delTime := ha.Keys[j].From; reflect.DeepEqual(emptyTime, delTime) {
						t.Errorf("key not deleted %v", delTime)
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
	for _, tc := range tests {
		tc := tc

		k2 := strings.ReplaceAll(tc.k2, "\n", "")
		if stripped := stripKey(tc.k1); stripped != k2 {
			t.Errorf("compareKeys(%q) = %q, want %q", tc.k1, stripped, k2)
		}
	}
}

func TestUpdateAll(t *testing.T) {
	t.Parallel()

	docContents := encodeKeys(key1, key2)

	cases := []struct {
		name  string
		delay time.Duration
		err   string
	}{
		{
			name:  "successful",
			delay: time.Duration(0),
			err:   "",
		},
		{
			name:  "failure",
			delay: 2 * time.Second,
			err:   "failed to processes ha.1: reading connection",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Set up the tc.
			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)
			haDB := hadb.New(testDB)
			mgr, err := NewManager(testDB, time.Minute, 1*time.Second, 2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tc.delay)
				fmt.Fprint(w, docContents)
				w.Header().Set("Content-Type", "application/json")
			}))
			defer ts.Close()

			// Add test health authorities.
			healthAuthorities := make([]*model.HealthAuthority, 0, 5)
			for i := 0; i < cap(healthAuthorities); i++ {
				ha := &model.HealthAuthority{
					Issuer:   fmt.Sprintf("iss.%d", i),
					Audience: "aud",
					Name:     fmt.Sprintf("ha.%d", i),
					JwksURI:  &ts.URL,
				}
				if err := haDB.AddHealthAuthority(ctx, ha); err != nil {
					t.Fatal(err)
				}
			}

			errcmp.MustMatch(t, mgr.UpdateAll(ctx), tc.err)
		})
	}
}
