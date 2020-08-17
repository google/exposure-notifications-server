package e2etest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	authorizedappdb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/database"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/sethvargo/go-envconfig"

	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
)

// newTestServer sets up client for local testing
func newE2ETest(tb testing.TB) *database.DB {
	tb.Helper()

	ctx := context.Background()

	// db := database.NewTestDatabase(tb)
	var db *database.DB
	if v := os.Getenv("DB_NAME"); v != "" && !testing.Short() {
		dbConfig := &database.Config{}
		sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
		if err != nil {
			tb.Fatalf("unable to connect to secret manager: %v", err)
		}
		if err := envconfig.ProcessWith(ctx, dbConfig, envconfig.OsLookuper(),
			secrets.Resolver(sm, &secrets.Config{})); err != nil {
			tb.Fatalf("error loading environment variables: %v", err)
		}

		db, err = database.NewFromEnv(ctx, dbConfig)
		if err != nil {
			tb.Fatalf("unable to connect to database: %v", err)
		}
	}
	return db
}

func publishKeys(payload *verifyapi.Publish, publishEndpoint string) (*verifyapi.PublishResponse, error) {
	j, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	resp, err := http.Post(publishEndpoint, "application/json", bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w: %s", err, body)
	}

	var pubResponse verifyapi.PublishResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

func checkResp(r *http.Response) ([]byte, error) {
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if r.StatusCode != 200 {
		return nil, fmt.Errorf("response was not 200 OK: %s", body)
	}

	return body, nil
}

func TestPublishEndpoint(t *testing.T) {
	exposureEndpoint := os.Getenv("EXPOSURE_URL")
	if exposureEndpoint == "" {
		t.Skip()
	}
	db := newE2ETest(t)
	keys := util.GenerateExposureKeys(3, -1, false)

	_, err := authorizedappdb.New(db).GetAuthorizedApp(context.Background(), "com.example.app")
	if err != nil {
		if err := authorizedappdb.New(db).InsertAuthorizedApp(context.Background(), &authorizedappmodel.AuthorizedApp{
			AppPackageName: "com.example.app",
			AllowedRegions: map[string]struct{}{
				"TEST": {},
			},
			AllowedHealthAuthorityIDs: map[int64]struct{}{
				1: {},
			},

			// TODO: hook up verification and revision
			BypassHealthAuthorityVerification: true,
			BypassRevisionToken:               true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Publish 3 keys
	payload := &verifyapi.Publish{
		Keys:              keys,
		HealthAuthorityID: "com.example.app",
	}
	resp, err := publishKeys(payload, exposureEndpoint+"/v1/publish")
	if err != nil {
		t.Fatalf("Failed publishing keys: \n\tResp: %v\n\t%v", resp, err)
	}

	criteria := publishdb.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}
	exposures, err := getExposures(db, criteria)
	if err != nil {
		t.Fatalf("Failed getting exposures: %v", err)
	}

	keysPublished := make(map[string]bool)
	for _, e := range exposures {
		strKey := base64.StdEncoding.EncodeToString(e.ExposureKey)
		keysPublished[strKey] = true
	}

	for _, want := range keys {
		if _, ok := keysPublished[want.Key]; !ok {
			t.Logf("Want published key %q not exist in exposures", want.Key)
		}
	}
}

// getExposures finds the exposures that match the given criteria.
func getExposures(db *database.DB, criteria publishdb.IterateExposuresCriteria) ([]*publishmodel.Exposure, error) {
	ctx := context.Background()
	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		return nil, err
	}

	return exposures, nil
}
