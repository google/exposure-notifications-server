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

// Package publish defines the exposure keys publishing API.
package publish

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifydb "github.com/google/exposure-notifications-server/internal/verification/database"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/mikehelmick/go-chaff"
)

// NewHandler creates the HTTP handler for the TTK publishing API.
func NewHandler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx)

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.AuthorizedAppProvider() == nil {
		return nil, fmt.Errorf("missing AuthorizedApp provider in server environment")
	}

	transformer, err := model.NewTransformer(config)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}
	logger.Infof("max keys per upload: %v", config.MaxKeysOnPublish)
	logger.Infof("max same start interval keys: %v", config.MaxSameStartIntervalKeys)
	logger.Infof("max interval start age: %v", config.MaxIntervalAge)
	logger.Infof("truncate window: %v", config.TruncateWindow)
	if config.DebugReleaseSameDayKeys() {
		logger.Warnf("SERVER IS IN DEBUG MODE. KEYS MAY BE RELEASED EARLY.")
	}

	verifier, err := verification.New(verifydb.New(env.Database()), &config.Verification)
	if err != nil {
		return nil, fmt.Errorf("verification.New: %w", err)
	}

	aadBytes, err := config.RevisionTokenADDBytes()
	if err != nil {
		return nil, fmt.Errorf("must provide ADD for revision token encryption in REVISION_TOKEN_AAD env variable: %w", err)
	}
	revisionKeyConfig := revisiondb.KMSConfig{
		WrapperKeyID: config.RevisionTokenKeyID,
		KeyManager:   env.GetKeyManager(),
	}
	revisionDB, err := revisiondb.New(env.Database(), &revisionKeyConfig)
	if err != nil {
		return nil, fmt.Errorf("revisiondb.New: %w", err)
	}
	tm, err := revision.New(ctx, revisionDB, config.RevisionKeyCacheDuration, config.RevisionTokenMinLength)
	if err != nil {
		return nil, fmt.Errorf("revision.New: %w", err)
	}

	return &publishHandler{
		serverenv:             env,
		transformer:           transformer,
		config:                config,
		database:              database.New(env.Database()),
		tracker:               chaff.New(),
		tokenManager:          tm,
		tokenAAD:              aadBytes,
		authorizedAppProvider: env.AuthorizedAppProvider(),
		verifier:              verifier,
	}, nil
}

type publishHandler struct {
	config                *Config
	serverenv             *serverenv.ServerEnv
	transformer           *model.Transformer
	database              *database.PublishDB
	tokenManager          *revision.TokenManager
	tracker               *chaff.Tracker
	tokenAAD              []byte
	authorizedAppProvider authorizedapp.Provider
	verifier              *verification.Verifier
}

type response struct {
	status      int
	pubResponse *verifyapi.PublishResponse
	metric      string
	count       int // metricCount
}

func (h *publishHandler) handleRequest(w http.ResponseWriter, r *http.Request) response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.publishHandler).handleRequest")
	defer span.End()

	logger := logging.FromContext(ctx)
	metrics := h.serverenv.MetricsExporter(ctx)

	var data verifyapi.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		message := fmt.Sprintf("error unmarshaling API call, code: %v: %v", code, err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status:      http.StatusBadRequest,
			pubResponse: &verifyapi.PublishResponse{Error: message},
			metric:      "publish-bad-json", count: 1}
	}

	appConfig, err := h.authorizedAppProvider.AppConfig(ctx, data.AppPackageName)
	if err != nil {
		// Config loaded, but app with that name isn't registered. This can also
		// happen if the app was recently registered but the cache hasn't been
		// refreshed.
		if errors.Is(err, authorizedapp.ErrAppNotFound) {
			message := fmt.Sprintf("unauthorized app: %v", data.AppPackageName)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return response{status: http.StatusUnauthorized,
				pubResponse: &verifyapi.PublishResponse{Error: message},
				metric:      "publish-app-not-authorized", count: 1}
		}

		// A higher-level configuration error occurred, likely while trying to read
		// from the database. This is retryable, although won't succeed if the error
		// isn't transient.
		// This message (and logging) will only contain the AppPkgName from the request
		// and no other data from the request.
		message := fmt.Sprintf("no AuthorizedApp, dropping data: %v", err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status:      http.StatusUnauthorized,
			pubResponse: &verifyapi.PublishResponse{Error: message},
			metric:      "publish-error-loading-authorizedapp",
			count:       1,
		}
	}

	// Verify the request is from a permitted region.
	for _, r := range data.Regions {
		if !appConfig.IsAllowedRegion(r) {
			err := fmt.Errorf("app %v tried to write to unauthorized region %v", appConfig.AppPackageName, r)
			message := fmt.Sprintf("verifying allowed regions: %v", err)
			span.SetStatus(trace.Status{Code: trace.StatusCodePermissionDenied, Message: message})
			return response{
				status:      http.StatusUnauthorized,
				pubResponse: &verifyapi.PublishResponse{Error: message},
				metric:      "publish-region-not-authorized",
				count:       1,
			}
		}
	}

	// Perform health authority certificat verification.
	verifiedClaims, err := h.verifier.VerifyDiagnosisCertificate(ctx, appConfig, &data)
	if err != nil {
		if appConfig.BypassHealthAuthorityVerification {
			logger.Warnf("bypassing health authority certificate verification for app: %v", appConfig.AppPackageName)
			metrics.WriteInt("publish-health-authority-verification-bypassed", true, 1)
		} else {
			message := fmt.Sprintf("unable to validate diagnosis verification: %v", err)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
			return response{
				status:      http.StatusUnauthorized,
				pubResponse: &verifyapi.PublishResponse{Error: message},
				metric:      "publish-bad-verification", count: 1}
		}
	}

	// Examine the revision token.
	var token *pb.RevisionTokenData
	if len(data.RevisionToken) != 0 {
		encryptedToken, err := base64util.DecodeString(data.RevisionToken)
		if err != nil {
			logger.Errorf("unable to decode revision token, proceeding without: %v", err)
		} else {
			token, err = h.tokenManager.UnmarshalRevisionToken(ctx, encryptedToken, h.tokenAAD)
			if err != nil {
				logger.Errorf("unable to unmarshall / descrypt revision token: %v", err)
				token = nil // just in case.
			}
		}
	}

	batchTime := time.Now()
	exposures, err := h.transformer.TransformPublish(ctx, &data, verifiedClaims, batchTime)
	if err != nil {
		message := fmt.Sprintf("unable to read request data: %v", err)
		logger.Errorf(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
		return response{
			status:      http.StatusBadRequest,
			pubResponse: &verifyapi.PublishResponse{Error: message},
			metric:      "publish-transform-fail", count: 1}
	}

	n, err := h.database.InsertAndReviseExposures(ctx, exposures, token, appConfig.BypassRevisionToken)
	if err != nil {
		message := fmt.Sprintf("error writing exposure record: %v", err)
		logger.Errorf(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status:      http.StatusInternalServerError,
			pubResponse: &verifyapi.PublishResponse{Error: http.StatusText(http.StatusInternalServerError)},
			metric:      "publish-db-write-error", count: 1}
	}

	// Build the new revision token. Union of existing token take + new exposures.
	var keep pb.RevisionTokenData
	if token != nil {
		// put existing tokens that aren't too old back in the token.
		retainInterval := model.IntervalNumber(batchTime.Add(-1 * h.config.MaxIntervalAge))
		for _, rk := range token.RevisableKeys {
			if rk.IntervalNumber+rk.IntervalCount >= retainInterval {
				keep.RevisableKeys = append(keep.RevisableKeys, rk)
			}
		}
	}
	newToken, err := h.tokenManager.MakeRevisionToken(ctx, &keep, exposures, h.tokenAAD)
	if err != nil {
		logger.Errorf("unable to make new revision token: %v", err)
	}

	message := fmt.Sprintf("Inserted %d exposures.", n)
	span.AddAttributes(trace.Int64Attribute("inserted_exposures", int64(n)))
	logger.Info(message)
	return response{
		status: http.StatusOK,
		pubResponse: &verifyapi.PublishResponse{
			RevisionToken:     base64.StdEncoding.EncodeToString(newToken),
			InsertedExposures: n,
		},
		metric: "publish-exposures-written",
		count:  n,
	}
}

// ServeHTTP handles the publish event. It tracks requests and can handle chaff
// requests when provided a request with the X-Chaff header.
func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := h.handleRequest(w, r)

			if response.metric != "" {
				ctx := r.Context()
				metrics := h.serverenv.MetricsExporter(ctx)
				metrics.WriteInt(response.metric, true, response.count)
			}

			data, err := json.Marshal(response.pubResponse)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "{\"error\": \"%v\"}", err.Error())
				return
			}
			w.WriteHeader(response.status)
			fmt.Fprintf(w, "%s", data)
		})).ServeHTTP(w, r)
}
