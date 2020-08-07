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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	aadb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	aamodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/pb"
	pubdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verdb "github.com/google/exposure-notifications-server/internal/verification/database"
	vermodel "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/jackc/pgx/v4"

	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type version int

const (
	useV1 = iota
	useV1Alpha1
)

var (
	versions = []version{useV1, useV1Alpha1}
)

type nameAssigner struct {
	baseAPK  string
	modifier int
	assigned string
}

func makeNameAssigner(base string) *nameAssigner {
	return &nameAssigner{
		baseAPK: base,
	}
}

func (n *nameAssigner) next() string {
	n.modifier++
	n.assigned = fmt.Sprintf("%s.%d", n.baseAPK, n.modifier)
	return n.assigned
}

func (n *nameAssigner) current() string {
	return n.assigned
}

func TestPublishWithBypass(t *testing.T) {
	t.Parallel()

	names := makeNameAssigner("com.example.health")
	issNames := makeNameAssigner("com.verification.server")
	regions := makeNameAssigner("R")

	cases := []struct {
		Name               string
		ContentType        string // if blank, application/json
		SigningKey         *testutil.SigningKey
		TestRegion         string
		HealthAuthority    *vermodel.HealthAuthority    // Automatically linked to keys.
		HealthAuthorityKey *vermodel.HealthAuthorityKey // Automatically linked to SigningKey
		AuthorizedApp      *aamodel.AuthorizedApp       // Automatically linked to health authorities.
		Publish            verifyapi.Publish
		Regions            []string
		JWTTiming          time.Duration
		ReportType         string
		WantTRAdjustment   []int
		Code               int
		Error              string
		ErrorCode          string
		SkipVersions       map[version]bool
	}{
		{
			Name:       "successful_insert_bypass_ha_verification",
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:              util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID: names.current(),
			},
			Regions: []string{regions.current()},
			Code:    http.StatusOK,
		},
		{
			Name:        "invalid_content_type",
			ContentType: "application/pdf",
			Code:        http.StatusUnsupportedMediaType,
			Error:       "content-type is not application/json",
		},
		{
			Name:       "defaulted_regions",
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:              util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID: names.current(),
			},
			Regions: []string{},
			Code:    http.StatusOK,
		},
		{
			Name:       "bad_health_authority_id",
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:              util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID: names.current() + "WRONG",
			},
			Regions:   []string{"US"},
			Code:      http.StatusUnauthorized,
			Error:     "unauthorized health authority",
			ErrorCode: "unknown_health_authority_id",
		},
		{
			Name:       "write_to_unauthorized_region",
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:              util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID: names.current(),
			},
			Regions:      []string{regions.current() + "X"},
			Code:         http.StatusUnauthorized,
			Error:        "tried to write to unauthorized region " + regions.current() + "X",
			SkipVersions: map[version]bool{useV1: true},
		},
		{
			Name:       "bad_HA_certificate",
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			Regions: []string{regions.current()},
			Code:    http.StatusUnauthorized,
			Error:   "unable to validate diagnosis verification: token contains an invalid number of segments",
		},
		{
			Name:       "valid_HA_certificate",
			SigningKey: testutil.GetSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   issNames.next(),
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Traveler:            true,
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			ReportType: verifyapi.ReportTypeConfirmed,
			Regions:    []string{}, // will receive defaults
			Code:       http.StatusOK,
		},
		{
			Name:       "valid_HA_certificate_with_overrides",
			SigningKey: testutil.GetSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   issNames.next(),
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 0, false),
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			Regions:          []string{regions.current()},
			ReportType:       verifyapi.ReportTypeConfirmed,
			WantTRAdjustment: []int{verifyapi.TransmissionRiskConfirmedStandard, verifyapi.TransmissionRiskConfirmedStandard}, // 2 entries, both override to confirmed
			Code:             http.StatusOK,
		},
		{
			Name:       "revise_with_cert",
			SigningKey: testutil.GetSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   issNames.next(),
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 0, false),
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			Regions:          []string{regions.current()},
			ReportType:       verifyapi.ReportTypeClinical,
			WantTRAdjustment: []int{verifyapi.TransmissionRiskClinical, verifyapi.TransmissionRiskClinical},
			Code:             http.StatusOK,
		},
		{
			Name:       "certificate in future",
			SigningKey: testutil.GetSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   issNames.next(),
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			Regions:   []string{regions.current()},
			JWTTiming: time.Hour,
			Code:      http.StatusUnauthorized,
			Error:     "unable to validate diagnosis verification: Token used before issued",
		},
		{
			Name:       "certificate expired",
			SigningKey: testutil.GetSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   issNames.next(),
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			TestRegion: regions.next(),
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = names.next()
				authApp.AllowedRegions[regions.current()] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				HealthAuthorityID:   names.current(),
				VerificationPayload: "totally not a JWT",
			},
			Regions:   []string{regions.current()},
			JWTTiming: -6 * time.Minute,
			Code:      http.StatusUnauthorized,
			Error:     "unable to validate diagnosis verification: token is expired by 1m",
		},
	}

	for _, ver := range versions {
		addVer := "v1_"
		if ver == useV1Alpha1 {
			addVer = "v1Alpha1_"
		}

		ctx := context.Background()
		// Database init for all modules that will be used.
		testDB := coredb.NewTestDatabase(t)
		// Make key manager
		kms, err := keys.NewInMemory(ctx)
		if err != nil {
			t.Fatalf("can't make kms: %v", err)
		}
		keyID := "rev"
		kms.AddEncryptionKey(keyID)
		tokenAAD := make([]byte, 16)
		if _, err := rand.Read(tokenAAD); err != nil {
			t.Fatalf("not enough entropy: %v", err)
		}
		// Configure revision keys.
		revDB, err := revisiondb.New(testDB, &revisiondb.KMSConfig{WrapperKeyID: keyID, KeyManager: kms})
		if err != nil {
			t.Fatalf("unable to create revision DB handle: %v", err)
		}
		if _, err := revDB.CreateRevisionKey(ctx); err != nil {
			t.Fatalf("unable to create revision key: %v", err)
		}

		for _, tc := range cases {
			if tc.SkipVersions[ver] {
				continue
			}

			t.Run(addVer+tc.Name, func(t *testing.T) {
				ctx = context.Background()

				// And set up publish handler up front.
				config := Config{}
				config.AuthorizedApp.CacheDuration = time.Nanosecond
				config.CreatedAtTruncateWindow = time.Second
				config.MaxKeysOnPublish = 20
				config.MaxSameStartIntervalKeys = 2
				config.MaxIntervalAge = 14 * 24 * time.Hour
				aaProvider, err := authorizedapp.NewDatabaseProvider(ctx, testDB, config.AuthorizedAppConfig())
				if err != nil {
					t.Fatal(err)
				}
				config.RevisionToken.AAD = tokenAAD
				config.RevisionToken.KeyID = keyID
				env := serverenv.New(ctx,
					serverenv.WithDatabase(testDB),
					serverenv.WithAuthorizedAppProvider(aaProvider),
					serverenv.WithKeyManager(kms))
				// Some config overrides for test.

				pubHandler, err := NewHandler(ctx, &config, env)
				if err != nil {
					t.Fatalf("unable to create publish handler: %v", err)
				}
				handler := pubHandler.Handle()
				if ver == useV1Alpha1 {
					handler = pubHandler.HandleV1Alpha1()
				}

				// See if there is a health authority to set up.
				if tc.HealthAuthority != nil {
					verDB := verdb.New(testDB)
					if err := verDB.AddHealthAuthority(ctx, tc.HealthAuthority); err != nil {
						t.Fatal(err)
					}
					if tc.HealthAuthorityKey != nil {
						if tc.SigningKey == nil {
							t.Fatal("test cases that have health authority keys registered must provide a siningKey as well")
						}
						// Join in the public key.
						tc.HealthAuthorityKey.PublicKeyPEM = tc.SigningKey.PublicKey
						if err := verDB.AddHealthAuthorityKey(ctx, tc.HealthAuthority, tc.HealthAuthorityKey); err != nil {
							t.Fatal(err)
						}
					}
				}

				// Insert the authorized app for the test case, if one exists.
				if tc.AuthorizedApp != nil {
					appDB := aadb.New(env.Database())
					// join in the health authority, if there is one for this test.
					if tc.HealthAuthority != nil {
						tc.AuthorizedApp.AllowedHealthAuthorityIDs[tc.HealthAuthority.ID] = struct{}{}
					}
					if err := appDB.InsertAuthorizedApp(ctx, tc.AuthorizedApp); err != nil {
						t.Fatal(err)
					}
				}
				pubDB := pubdb.New(env.Database())

				// If verification is being used. The JWT and HMAC Salt must be incorporated.
				if tc.HealthAuthority != nil {
					cfg := testutil.JWTConfig{
						HealthAuthority:    tc.HealthAuthority,
						HealthAuthorityKey: tc.HealthAuthorityKey,
						ExposureKeys:       tc.Publish.Keys,
						Key:                tc.SigningKey.Key,
						JWTWarp:            tc.JWTTiming,
						ReportType:         tc.ReportType,
					}
					verification, salt := testutil.IssueJWT(t, cfg)
					tc.Publish.VerificationPayload = verification
					tc.Publish.HMACKey = salt
				}

				// Marshal the provided publish request.
				var jsonString []byte
				if ver == useV1 {
					jsonString, err = json.Marshal(tc.Publish)
				} else {
					publish := v1alpha1.Publish{
						Regions:              tc.Regions,
						AppPackageName:       tc.Publish.HealthAuthorityID,
						VerificationPayload:  tc.Publish.VerificationPayload,
						HMACKey:              tc.Publish.HMACKey,
						SymptomOnsetInterval: tc.Publish.SymptomOnsetInterval,
						RevisionToken:        tc.Publish.RevisionToken,
						Padding:              tc.Publish.Padding,
					}
					publish.Keys = make([]v1alpha1.ExposureKey, len(tc.Publish.Keys))
					for i, k := range tc.Publish.Keys {
						publish.Keys[i] = v1alpha1.ExposureKey{
							Key:              k.Key,
							IntervalNumber:   k.IntervalNumber,
							IntervalCount:    k.IntervalCount,
							TransmissionRisk: k.TransmissionRisk,
						}
					}
					jsonString, err = json.Marshal(publish)
				}
				if err != nil {
					t.Fatal(err)
				}

				server := httptest.NewServer(handler)
				defer server.Close()

				// make the request
				contentType := "application/json"
				if tc.ContentType != "" {
					contentType = tc.ContentType
				}
				resp, err := server.Client().Post(server.URL, contentType, strings.NewReader(string(jsonString)))
				if err != nil {
					t.Fatal(err)
				}

				// For non success status, check that they body contains the expected message
				defer resp.Body.Close()
				respBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				log.Printf("\n\n%#v\n\n", string(respBytes))

				var response verifyapi.PublishResponse
				if err := json.Unmarshal(respBytes, &response); err != nil {
					t.Fatalf("unable to unmarshal response body: %v; data: %v", err, string(respBytes))
				}
				if resp.StatusCode != tc.Code {
					t.Fatalf("http response code want: %v, got %v.", tc.Code, resp.StatusCode)
				}

				if ver == useV1 {
					// The extended data validation only happens on verifyapi.

					if resp.StatusCode == http.StatusOK {
						// For success requests, verify that the exposures were inserted.
						criteria := pubdb.IterateExposuresCriteria{
							IncludeRegions: []string{tc.TestRegion},
							SinceTimestamp: time.Now().Add(-1 * time.Minute),
							UntilTimestamp: time.Now().Add(time.Minute),
						}

						got := make([]*model.Exposure, 0, len(tc.Publish.Keys))
						_, err = pubDB.IterateExposures(ctx, criteria, func(ex *model.Exposure) error {
							got = append(got, ex)
							return nil
						})
						if err != nil {
							t.Fatal(err)
						}

						// In v1 regions get joined in from the HA or supplemental data from a v1alpha1 upgrade.
						wantRegions := tc.Regions
						if len(wantRegions) == 0 {
							wantRegions = tc.AuthorizedApp.AllAllowedRegions()
						}

						want := make([]*model.Exposure, 0, len(tc.Publish.Keys))
						tokenWant := &pb.RevisionTokenData{}
						for _, k := range tc.Publish.Keys {
							if key, err := base64util.DecodeString(k.Key); err != nil {
								t.Fatal(err)
							} else {
								next := model.Exposure{
									ExposureKey:      key,
									AppPackageName:   tc.Publish.HealthAuthorityID,
									TransmissionRisk: k.TransmissionRisk,
									IntervalNumber:   k.IntervalNumber,
									IntervalCount:    k.IntervalCount,
									Regions:          wantRegions,
									Traveler:         tc.Publish.Traveler,
									LocalProvenance:  true,
									FederationSyncID: 0,
								}
								if tc.ReportType != "" {
									next.ReportType = tc.ReportType
								}
								if tc.HealthAuthority != nil {
									next.SetHealthAuthorityID(tc.HealthAuthority.ID)
								}

								want = append(want, &next)

								tokenWant.RevisableKeys = append(tokenWant.RevisableKeys,
									&pb.RevisableKey{
										TemporaryExposureKey: key,
										IntervalNumber:       k.IntervalNumber,
										IntervalCount:        k.IntervalCount,
									})
							}
						}

						// Adjust expectations based on transmission risk overrides placed in JWT.
						for i, a := range tc.WantTRAdjustment {
							want[i].TransmissionRisk = a
						}

						sorter := cmp.Transformer("Sort", func(in []*model.Exposure) []*model.Exposure {
							out := append([]*model.Exposure(nil), in...) // Copy input to avoid mutating it
							sort.Slice(out, func(i int, j int) bool {
								return bytes.Compare(out[i].ExposureKey, out[j].ExposureKey) <= 0
							})
							return out
						})
						ignoreCreatedAt := cmpopts.IgnoreFields(*want[0], "CreatedAt")

						if diff := cmp.Diff(want, got, sorter, ignoreCreatedAt, cmpopts.IgnoreUnexported(model.Exposure{})); diff != "" {
							t.Errorf("mismatch (-want, +got):\n%s", diff)
						}

						// Crack the revision token - it should contain the uploaded exposure keys.
						tm, err := revision.New(ctx, revDB, time.Minute, 1)
						if err != nil {
							t.Fatalf("unable to create token manager: %v", err)
						}
						revTokenBytes, err := base64util.DecodeString(response.RevisionToken)
						if err != nil {
							t.Fatalf("revision token encoded incorrectly: %v", err)
						}
						revToken, err := tm.UnmarshalRevisionToken(ctx, revTokenBytes, tokenAAD)
						if err != nil {
							t.Fatalf("unable to decrypt revision token: %v", err)
						}
						revisableSorter := cmp.Transformer("Sort", func(in []*pb.RevisableKey) []*pb.RevisableKey {
							out := append([]*pb.RevisableKey(nil), in...)
							sort.Slice(out, func(i int, j int) bool {
								return bytes.Compare(out[i].TemporaryExposureKey, out[j].TemporaryExposureKey) <= 0
							})
							return out
						})
						if diff := cmp.Diff(tokenWant.RevisableKeys, revToken.RevisableKeys, revisableSorter, cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
							t.Errorf("mismatch (-want, +got):\n%s", diff)
						}
					} else {
						if !strings.Contains(response.ErrorMessage, tc.Error) {
							t.Errorf("missing error text '%v', got '%+v'", tc.Error, response)
						}
						if tc.ErrorCode != "" && response.Code != tc.ErrorCode {
							t.Errorf("wrong error code want: %v, got: %v", tc.ErrorCode, response.Code)
						}
					}
				}
			})
		}
	}
}

type RevisionTokenChanger func(ctx context.Context, token string, tm *revision.TokenManager, aad []byte) string

func TokenIdentity(ctx context.Context, token string, tm *revision.TokenManager, aad []byte) string {
	return token
}

func TestKeyRevision(t *testing.T) {
	t.Parallel()

	haName := "gov.state.health"
	region := "US"

	signingKey := testutil.GetSigningKey(t)
	authorizedApp := func() *aamodel.AuthorizedApp {
		authApp := aamodel.NewAuthorizedApp()
		authApp.AppPackageName = haName
		authApp.BypassHealthAuthorityVerification = true
		authApp.BypassRevisionToken = false
		authApp.AllowedRegions[region] = struct{}{}
		return authApp
	}()
	healthAuthority := &vermodel.HealthAuthority{
		Issuer:   "gov.state.health",
		Audience: "unit.test.server",
		Name:     "State Dept of Health",
	}
	healthAuthorityKey := &vermodel.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}

	ctx := context.Background()
	// Database init for all modules that will be used.
	testDB := coredb.NewTestDatabase(t)
	// Make key manager
	kms, err := keys.NewInMemory(ctx)
	if err != nil {
		t.Fatalf("can't make kms: %v", err)
	}
	keyID := "rev"
	kms.AddEncryptionKey(keyID)
	tokenAAD := make([]byte, 16)
	if _, err := rand.Read(tokenAAD); err != nil {
		t.Fatalf("not enough entropy: %v", err)
	}
	// Configure revision keys.
	revDB, err := revisiondb.New(testDB, &revisiondb.KMSConfig{WrapperKeyID: keyID, KeyManager: kms})
	if err != nil {
		t.Fatalf("unable to create revision DB handle: %v", err)
	}
	if _, err := revDB.CreateRevisionKey(ctx); err != nil {
		t.Fatalf("unable to create revision key: %v", err)
	}

	// And set up publish handler up front.
	config := Config{}
	config.AuthorizedApp.CacheDuration = time.Nanosecond
	config.CreatedAtTruncateWindow = time.Second
	config.MaxKeysOnPublish = 20
	config.MaxSameStartIntervalKeys = 2
	config.MaxIntervalAge = 14 * 24 * time.Hour
	aaProvider, err := authorizedapp.NewDatabaseProvider(ctx, testDB, config.AuthorizedAppConfig())
	if err != nil {
		t.Fatal(err)
	}
	config.RevisionToken.AAD = tokenAAD
	config.RevisionToken.KeyID = keyID
	env := serverenv.New(ctx,
		serverenv.WithDatabase(testDB),
		serverenv.WithAuthorizedAppProvider(aaProvider),
		serverenv.WithKeyManager(kms))

	tm, err := revision.New(ctx, revDB, time.Duration(0), 0)
	if err != nil {
		t.Fatalf("unable to create token manager: %v", err)
	}

	verDB := verdb.New(testDB)
	if err := verDB.AddHealthAuthority(ctx, healthAuthority); err != nil {
		t.Fatal(err)
	}

	// Join in the public key.
	healthAuthorityKey.PublicKeyPEM = signingKey.PublicKey
	if err := verDB.AddHealthAuthorityKey(ctx, healthAuthority, healthAuthorityKey); err != nil {
		t.Fatal(err)

	}

	appDB := aadb.New(env.Database())
	authorizedApp.AllowedHealthAuthorityIDs[healthAuthority.ID] = struct{}{}
	if err := appDB.InsertAuthorizedApp(ctx, authorizedApp); err != nil {
		t.Fatal(err)
	}

	pubHandler, err := NewHandler(ctx, &config, env)
	if err != nil {
		t.Fatalf("unable to create publish handler: %v", err)
	}
	handler := pubHandler.Handle()
	server := httptest.NewServer(handler)
	defer server.Close()

	pubDB := pubdb.New(testDB)

	cases := []struct {
		Name           string
		Publish        verifyapi.Publish
		ErrorCode      string
		RevErrorCode   string
		RevTokenMesser RevisionTokenChanger
	}{
		{
			Name: "normal_revision",
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 0, false),
				Traveler:            false,
				HealthAuthorityID:   haName,
				VerificationPayload: "totally not a JWT",
			},
			RevTokenMesser: TokenIdentity,
		},
		{
			Name: "missing_revision_token",
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 0, false),
				Traveler:            false,
				HealthAuthorityID:   haName,
				VerificationPayload: "totally not a JWT",
			},
			RevErrorCode: verifyapi.ErrorMissingRevisionToken,
			RevTokenMesser: func(ctx context.Context, token string, tm *revision.TokenManager, aad []byte) string {
				return ""
			},
		},
		{
			Name: "token_missing_keys",
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 0, false),
				Traveler:            false,
				HealthAuthorityID:   haName,
				VerificationPayload: "totally not a JWT",
			},
			RevErrorCode: verifyapi.ErrorInvalidRevisionToken,
			RevTokenMesser: func(ctx context.Context, token string, tm *revision.TokenManager, aad []byte) string {
				tokenBytes, err := base64util.DecodeString(token)
				if err != nil {
					return ""
				}
				revToken, err := tm.UnmarshalRevisionToken(ctx, tokenBytes, aad)
				if err != nil {
					return ""
				}
				// Gotta throw some new keys in, or we can't mint a new revision token.
				newKeys := []*model.Exposure{
					{
						ExposureKey:    make([]byte, 16),
						IntervalCount:  1,
						IntervalNumber: 1,
					},
				}
				revToken.RevisableKeys = revToken.RevisableKeys[0:1]
				tokenBytes, err = tm.MakeRevisionToken(ctx, revToken, newKeys, aad)
				if err != nil {
					return ""
				}
				return base64.StdEncoding.EncodeToString(tokenBytes)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx = context.Background()

			revisionToken := ""
			// Do the initial insert
			{
				// Issue the likely diagnosis certificate.
				cfg := testutil.JWTConfig{
					HealthAuthority:    healthAuthority,
					HealthAuthorityKey: healthAuthorityKey,
					ExposureKeys:       tc.Publish.Keys,
					Key:                signingKey.Key,
					JWTWarp:            time.Duration(0),
					ReportType:         verifyapi.ReportTypeClinical,
				}
				verification, salt := testutil.IssueJWT(t, cfg)
				tc.Publish.VerificationPayload = verification
				tc.Publish.HMACKey = salt

				// Marshal the provided publish request.
				jsonString, err := json.Marshal(tc.Publish)
				if err != nil {
					t.Fatal(err)
				}

				// make the initial request
				resp, err := server.Client().Post(server.URL, "application/json", strings.NewReader(string(jsonString)))
				if err != nil {
					t.Fatal(err)
				}

				// For non success status, check that they body contains the expected message
				defer resp.Body.Close()
				respBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				var response verifyapi.PublishResponse
				if err := json.Unmarshal(respBytes, &response); err != nil {
					t.Fatalf("unable to unmarshal response body: %v; data: %v", err, string(respBytes))
				}

				if response.Code != tc.ErrorCode {
					t.Fatalf("wrong code on initial publish, want %v, got %v", tc.ErrorCode, response.Code)
				}
				revisionToken = response.RevisionToken
			}

			// Make the revision.
			{
				revisionToken = tc.RevTokenMesser(ctx, revisionToken, tm, tokenAAD)

				cfg := testutil.JWTConfig{
					HealthAuthority:    healthAuthority,
					HealthAuthorityKey: healthAuthorityKey,
					ExposureKeys:       tc.Publish.Keys,
					Key:                signingKey.Key,
					JWTWarp:            time.Duration(0),
					ReportType:         verifyapi.ReportTypeConfirmed,
				}
				verification, salt := testutil.IssueJWT(t, cfg)
				tc.Publish.VerificationPayload = verification
				tc.Publish.HMACKey = salt

				// Add the revision token to publish request.
				tc.Publish.RevisionToken = revisionToken

				// Marshal the provided publish request.
				jsonString, err := json.Marshal(tc.Publish)
				if err != nil {
					t.Fatal(err)
				}

				// make the initial request
				resp, err := server.Client().Post(server.URL, "application/json", strings.NewReader(string(jsonString)))
				if err != nil {
					t.Fatal(err)
				}

				// For non success status, check that they body contains the expected message
				defer resp.Body.Close()
				respBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				var response verifyapi.PublishResponse
				if err := json.Unmarshal(respBytes, &response); err != nil {
					t.Fatalf("unable to unmarshal response body: %v; data: %v", err, string(respBytes))
				}

				if response.Code != tc.RevErrorCode {
					t.Fatalf("wrong code on initial publish, want %v, got %v", tc.ErrorCode, response.Code)
				}
			}

			// If the test case expects revision to be successful, read back the TEKs.
			if tc.RevErrorCode == "" {
				expectedKeys := make([]string, len(tc.Publish.Keys))
				want := make(map[string]*model.Exposure)
				revisedReportType := verifyapi.ReportTypeConfirmed
				revisedTransmissionRisk := verifyapi.TransmissionRiskConfirmedStandard
				for i, k := range tc.Publish.Keys {
					expectedKeys[i] = k.Key
					keyBytes, err := base64util.DecodeString(k.Key)
					if err != nil {
						t.Fatalf("unable to decode exposure key: %v", err)
					}
					want[k.Key] = &model.Exposure{
						ExposureKey:             keyBytes,
						TransmissionRisk:        verifyapi.TransmissionRiskClinical,
						AppPackageName:          haName,
						Regions:                 []string{region},
						Traveler:                false,
						IntervalNumber:          k.IntervalNumber,
						IntervalCount:           k.IntervalCount,
						LocalProvenance:         true,
						HealthAuthorityID:       &healthAuthority.ID,
						ReportType:              verifyapi.ReportTypeClinical,
						DaysSinceSymptomOnset:   nil,
						RevisedReportType:       &revisedReportType,
						RevisedTransmissionRisk: &revisedTransmissionRisk,
					}
				}

				var got map[string]*model.Exposure
				var err error

				testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
					got, err = pubDB.ReadExposures(ctx, tx, expectedKeys)
					return err
				})

				ignoreCreatedAt := cmpopts.IgnoreFields(model.Exposure{}, "CreatedAt", "RevisedAt")
				if diff := cmp.Diff(want, got, ignoreCreatedAt, cmpopts.IgnoreUnexported(model.Exposure{})); diff != "" {
					t.Errorf("mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}
