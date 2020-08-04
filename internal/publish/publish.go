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
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifydb "github.com/google/exposure-notifications-server/internal/verification/database"
	v1 "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/mikehelmick/go-chaff"
)

// newProcessor creates common API handler for the publish API.
// This uses the latest version of the API and it is the responsibility of the
// specific version handler to upgrade requests and downgrade responses.
func newProcessor(ctx context.Context, config *Config, env *serverenv.ServerEnv) (*publishHandler, error) {
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

	aadBytes := config.RevisionToken.AAD
	if len(aadBytes) == 0 {
		return nil, fmt.Errorf("must provide ADD for revision token encryption in REVISION_TOKEN_AAD env variable: %w", err)
	}
	revisionKeyConfig := revisiondb.KMSConfig{
		WrapperKeyID: config.RevisionToken.KeyID,
		KeyManager:   env.GetKeyManager(),
	}
	revisionDB, err := revisiondb.New(env.Database(), &revisionKeyConfig)
	if err != nil {
		return nil, fmt.Errorf("revisiondb.New: %w", err)
	}
	tm, err := revision.New(ctx, revisionDB, config.RevisionKeyCacheDuration, config.RevisionToken.MinLength)
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
	pubResponse *v1.PublishResponse
	metric      string
	count       int // metricCount
}

// versionBridge closes the gap in up-leveling v1alpha1 to v1 API.
type versionBridge struct {
	AdditionalRegions []string
}

func newVersionBridge(regions []string) *versionBridge {
	b := versionBridge{
		AdditionalRegions: make([]string, len(regions)),
	}
	copy(b.AdditionalRegions, regions)
	return &b
}

// process runs the publish business logic over a "v1" version of the publish request
// and knows how to join in data from previous versions (the provided versionBridge)
func (h *publishHandler) process(ctx context.Context, data *v1.Publish, bridge *versionBridge) response {
	ctx, span := trace.StartSpan(ctx, "(*publish.publishHandler).process")
	defer span.End()

	logger := logging.FromContext(ctx)
	metrics := h.serverenv.MetricsExporter(ctx)

	appConfig, err := h.authorizedAppProvider.AppConfig(ctx, data.HealthAuthorityID)
	if err != nil {
		// Config loaded, but app with that name isn't registered. This can also
		// happen if the app was recently registered but the cache hasn't been
		// refreshed.
		if errors.Is(err, authorizedapp.ErrAppNotFound) {
			message := fmt.Sprintf("unauthorized health authority: %v", data.HealthAuthorityID)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return response{
				status: http.StatusUnauthorized,
				pubResponse: &v1.PublishResponse{
					ErrorMessage: message,
					ErrorCode:    v1.ErrorUnknownHealthAuthorityID,
				},
				metric: "publish-health-authority-not-authorized",
				count:  1,
			}
		}

		// A higher-level configuration error occurred, likely while trying to read
		// from the database. This is retryable, although won't succeed if the error
		// isn't transient.
		// This message (and logging) will only contain the AppPkgName from the request
		// and no other data from the request.
		message := fmt.Sprintf("error loading health authority config: %v", err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status: http.StatusNotFound,
			pubResponse: &v1.PublishResponse{
				ErrorMessage: message,
				ErrorCode:    v1.ErrorUnableToLoadHealthAuthority,
			},
			metric: "publish-error-loading-authorizedapp",
			count:  1,
		}
	}

	// In the v1 API - regions aren't passed. They may be passed from v1Apha1
	var regions []string
	if bridge != nil && len(bridge.AdditionalRegions) > 0 {
		regions = bridge.AdditionalRegions
		// Authorized regions only need to checked if they are coming in from the
		// v1alpha1 version of the API.
		for _, r := range regions {
			if !appConfig.IsAllowedRegion(r) {
				err := fmt.Errorf("app %v tried to write to unauthorized region %v", appConfig.AppPackageName, r)
				message := fmt.Sprintf("verifying allowed regions: %v", err)
				span.SetStatus(trace.Status{Code: trace.StatusCodePermissionDenied, Message: message})
				return response{
					status: http.StatusUnauthorized,
					pubResponse: &v1.PublishResponse{
						ErrorMessage: message, // Error code omitted, since this isn't in the v1 path.
					},
					metric: "publish-region-not-authorized",
					count:  1,
				}
			}
		}
	}
	// If regions is still empty (normal for v1 request), copy the regions from
	// the authorized app config.
	if len(regions) == 0 {
		regions = appConfig.AllAllowedRegions()
	}
	// And - worse case, still no regions and server default set.
	if len(regions) == 0 && h.config.DefaultRegion != "" {
		regions = append(regions, h.config.DefaultRegion)
	}

	// Verify that there is at least one region set by API call or by one of the
	// generous defaults. If there isn't a region set, then the TEKs
	// won't be retrievable, so we ensure there is something set.
	if len(regions) == 0 {
		message := fmt.Sprintf("unknown health authority regions for %v", data.HealthAuthorityID)
		span.SetStatus(trace.Status{Code: trace.StatusCodePermissionDenied, Message: message})
		return response{
			status: http.StatusBadRequest,
			pubResponse: &v1.PublishResponse{
				ErrorMessage: message,
				ErrorCode:    v1.ErrorHealthAuthorityMissingRegionConfiguration,
			},
			metric: "publish-region-not-specified",
			count:  1,
		}
	}

	// Perform health authority certificate verification.
	verifiedClaims, err := h.verifier.VerifyDiagnosisCertificate(ctx, appConfig, data)
	if err != nil {
		if appConfig.BypassHealthAuthorityVerification {
			logger.Warnf("bypassing health authority certificate verification health authority: %v", appConfig.AppPackageName)
			metrics.WriteInt("publish-health-authority-verification-bypassed", true, 1)
		} else {
			message := fmt.Sprintf("unable to validate diagnosis verification: %v", err)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
			return response{
				status: http.StatusUnauthorized,
				pubResponse: &v1.PublishResponse{
					ErrorMessage: message,
					ErrorCode:    v1.ErrorVerificationCertificateInvalid,
				},
				metric: "publish-bad-verification", count: 1}
		}
	}

	// Examine the revision token. It is expected that it is missing in most cases.
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
	exposures, err := h.transformer.TransformPublish(ctx, data, regions, verifiedClaims, batchTime)
	if err != nil {
		message := fmt.Sprintf("unable to read request data: %v", err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
		return response{
			status: http.StatusBadRequest,
			pubResponse: &v1.PublishResponse{
				ErrorMessage: message,
				ErrorCode:    v1.ErrorBadRequest,
			},
			metric: "publish-transform-fail", count: 1}
	}

	n, err := h.database.InsertAndReviseExposures(ctx, exposures, token, !appConfig.BypassRevisionToken)
	if err != nil {
		message := fmt.Sprintf("error writing exposure record: %v", err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status: http.StatusInternalServerError,
			pubResponse: &v1.PublishResponse{
				ErrorMessage: http.StatusText(http.StatusInternalServerError),
				ErrorCode:    v1.ErrorInternalError,
			},
			metric: "publish-db-write-error", count: 1}
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
		// In this case, an empty revision token is returned. The rest of the request
		// was handled correctly.
		logger.Errorf("unable to make new revision token: %v", err)
		newToken = make([]byte, 0)
	}

	message := fmt.Sprintf("Inserted %d exposures.", n)
	span.AddAttributes(trace.Int64Attribute("inserted_exposures", int64(n)))
	logger.Info(message)
	return response{
		status: http.StatusOK,
		pubResponse: &v1.PublishResponse{
			RevisionToken:     base64.StdEncoding.EncodeToString(newToken),
			InsertedExposures: n,
		},
		metric: "publish-exposures-written",
		count:  n,
	}
}
