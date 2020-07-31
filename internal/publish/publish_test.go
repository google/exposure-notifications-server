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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
	"github.com/google/exposure-notifications-server/internal/util"
	verdb "github.com/google/exposure-notifications-server/internal/verification/database"
	vermodel "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/keys"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	utils "github.com/google/exposure-notifications-server/pkg/verification"

	"github.com/dgrijalva/jwt-go"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Holds a single signing key and the PEM public key.
// Each test case has it's own key issued.
type signingKey struct {
	Key       *ecdsa.PrivateKey
	PublicKey string
}

func newSigningKey(t *testing.T) *signingKey {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	publicKey := privateKey.Public()
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})
	pemPublicKey := string(pemEncodedPub)

	return &signingKey{
		Key:       privateKey,
		PublicKey: pemPublicKey,
	}
}

type jwtConfig struct {
	HealthAuthority    *vermodel.HealthAuthority
	HealthAuthorityKey *vermodel.HealthAuthorityKey
	Publish            *verifyapi.Publish
	Key                *ecdsa.PrivateKey
	JWTWarp            time.Duration
	Overrides          verifyapi.TransmissionRiskVector
}

// Based on the publish request, generate a JWT as if it came from the
// authorized health authority.
func issueJWT(t *testing.T, cfg jwtConfig) (jwtText, hmacKey string) {
	t.Helper()

	hmacKeyBytes := make([]byte, 32)
	if _, err := rand.Read(hmacKeyBytes); err != nil {
		t.Fatal(err)
	}
	hmacKey = base64.StdEncoding.EncodeToString(hmacKeyBytes)

	hmacBytes, err := utils.CalculateExposureKeyHMAC(cfg.Publish.Keys, hmacKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	hmac := base64.StdEncoding.EncodeToString(hmacBytes)

	claims := verifyapi.NewVerificationClaims()
	claims.Audience = cfg.HealthAuthority.Audience
	claims.Issuer = cfg.HealthAuthority.Issuer
	claims.IssuedAt = time.Now().Add(cfg.JWTWarp).Unix()
	claims.ExpiresAt = time.Now().Add(cfg.JWTWarp).Add(5 * time.Minute).Unix()
	claims.SignedMAC = hmac
	claims.TransmissionRisks = cfg.Overrides

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header[verifyapi.KeyIDHeader] = cfg.HealthAuthorityKey.Version
	jwtText, err = token.SignedString(cfg.Key)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestPublishWithBypass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name               string
		ContentType        string // if blank, application/json
		SigningKey         *signingKey
		HealthAuthority    *vermodel.HealthAuthority    // Automatically linked to keys.
		HealthAuthorityKey *vermodel.HealthAuthorityKey // Automatically linked to SigningKey
		AuthorizedApp      *aamodel.AuthorizedApp       // Automatically linked to health authorities.
		Publish            verifyapi.Publish
		JWTTiming          time.Duration
		Overrides          verifyapi.TransmissionRiskVector
		WantTRAdjustment   []int
		Code               int
		Error              string
	}{
		{
			Name: "successful insert, bypass HA verification",
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:           util.GenerateExposureKeys(2, 5, false),
				Regions:        []string{"US"},
				AppPackageName: "com.example.health",
			},
			Code: http.StatusOK,
		},
		{
			Name:        "invalid content type",
			ContentType: "application/pdf",
			Code:        http.StatusBadRequest,
			Error:       "content-type is not application/json",
		},
		{
			Name: "missing_regions",
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:           util.GenerateExposureKeys(2, 5, false),
				Regions:        []string{},
				AppPackageName: "com.example.health",
			},
			Code:  http.StatusBadRequest,
			Error: "no regions provided",
		},
		{
			Name: "bad app package name",
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:           util.GenerateExposureKeys(2, 5, false),
				Regions:        []string{"US"},
				AppPackageName: "com.example.health.WRONG",
			},
			Code:  http.StatusUnauthorized,
			Error: "unauthorized app",
		},
		{
			Name: "write to unauthorized region",
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.BypassHealthAuthorityVerification = true
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:           util.GenerateExposureKeys(2, 5, false),
				Regions:        []string{"CA"},
				AppPackageName: "com.example.health",
			},
			Code:  http.StatusUnauthorized,
			Error: "tried to write to unauthorized region CA",
		},
		{
			Name: "bad HA certificate",
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Regions:             []string{"US"},
				AppPackageName:      "com.example.health",
				VerificationPayload: "totally not a JWT",
			},
			Code:  http.StatusUnauthorized,
			Error: "unable to validate diagnosis verification: token contains an invalid number of segments",
		},
		{
			Name:       "valid_HA_certificate",
			SigningKey: newSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   "doh.my.gov",
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Regions:             []string{"US"},
				Traveler:            true,
				AppPackageName:      "com.example.health",
				VerificationPayload: "totally not a JWT",
			},
			Code: http.StatusOK,
		},
		{
			Name:       "valid_HA_certificate_with_overrides",
			SigningKey: newSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   "doh.my.gov",
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Regions:             []string{"US"},
				AppPackageName:      "com.example.health",
				VerificationPayload: "totally not a JWT",
			},
			Overrides: []verifyapi.TransmissionRiskOverride{
				{
					TransmissionRisk:     8,
					SinceRollingInterval: 0,
				},
			},
			WantTRAdjustment: []int{8, 8}, // 2 entries, both override to 8
			Code:             http.StatusOK,
		},
		{
			Name:       "certificate in future",
			SigningKey: newSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   "doh.my.gov",
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Regions:             []string{"US"},
				AppPackageName:      "com.example.health",
				VerificationPayload: "totally not a JWT",
			},
			JWTTiming: time.Hour,
			Code:      http.StatusUnauthorized,
			Error:     "unable to validate diagnosis verification: Token used before issued",
		},
		{
			Name:       "certificate expired",
			SigningKey: newSigningKey(t),
			HealthAuthority: &vermodel.HealthAuthority{
				Issuer:   "doh.my.gov",
				Audience: "unit.test.server",
				Name:     "Unit Test Gov DOH",
			},
			HealthAuthorityKey: &vermodel.HealthAuthorityKey{
				Version: "v1",
				From:    time.Now().Add(-1 * time.Minute),
			},
			AuthorizedApp: func() *aamodel.AuthorizedApp {
				authApp := aamodel.NewAuthorizedApp()
				authApp.AppPackageName = "com.example.health"
				authApp.AllowedRegions["US"] = struct{}{}
				return authApp
			}(),
			Publish: verifyapi.Publish{
				Keys:                util.GenerateExposureKeys(2, 5, false),
				Regions:             []string{"US"},
				AppPackageName:      "com.example.health",
				VerificationPayload: "totally not a JWT",
			},
			JWTTiming: -6 * time.Minute,
			Code:      http.StatusUnauthorized,
			Error:     "unable to validate diagnosis verification: token is expired by 1m",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Database init for all modules that will be used.
			testDB := coredb.NewTestDatabase(t)
			ctx := context.Background()

			// Make key manager
			kms, err := keys.NewInMemory(ctx)
			if err != nil {
				t.Fatalf("can't make kms: %v", err)
			}
			keyID := "rev" + tc.Name
			kms.AddEncryptionKey(keyID)
			tokenAAD := make([]byte, 16)
			if _, err := rand.Read(tokenAAD); err != nil {
				t.Fatalf("not enough entropy: %v", err)
			}
			aad := base64.StdEncoding.EncodeToString(tokenAAD)
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
			config.RevisionTokenAAD = aad
			config.RevisionTokenKeyID = keyID
			env := serverenv.New(ctx,
				serverenv.WithDatabase(testDB),
				serverenv.WithAuthorizedAppProvider(aaProvider),
				serverenv.WithKeyManager(kms))
			// Some config overrides for test.
			handler, err := NewHandler(ctx, &config, env)
			if err != nil {
				t.Fatal(err)
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
				cfg := jwtConfig{
					HealthAuthority:    tc.HealthAuthority,
					HealthAuthorityKey: tc.HealthAuthorityKey,
					Publish:            &tc.Publish,
					Key:                tc.SigningKey.Key,
					JWTWarp:            tc.JWTTiming,
					Overrides:          tc.Overrides,
				}
				verification, salt := issueJWT(t, cfg)
				tc.Publish.VerificationPayload = verification
				tc.Publish.HMACKey = salt
			}

			// Marshal the provided publish request.
			jsonString, err := json.Marshal(tc.Publish)
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

			if resp.StatusCode == http.StatusOK {
				// For success requests, verify that the exposures were inserted.
				criteria := pubdb.IterateExposuresCriteria{
					IncludeRegions: []string{"US"},
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

				want := make([]*model.Exposure, 0, len(tc.Publish.Keys))
				tokenWant := &pb.RevisionTokenData{}
				for _, k := range tc.Publish.Keys {
					if key, err := base64util.DecodeString(k.Key); err != nil {
						t.Fatal(err)
					} else {
						next := model.Exposure{
							ExposureKey:      key,
							AppPackageName:   tc.Publish.AppPackageName,
							TransmissionRisk: k.TransmissionRisk,
							IntervalNumber:   k.IntervalNumber,
							IntervalCount:    k.IntervalCount,
							Regions:          tc.Publish.Regions,
							Traveler:         tc.Publish.Traveler,
							LocalProvenance:  true,
							FederationSyncID: 0,
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
				if !strings.Contains(response.Error, tc.Error) {
					t.Errorf("missing error text '%v', got '%+v'", tc.Error, response)
				}
			}
		})
	}
}
