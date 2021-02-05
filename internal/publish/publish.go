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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifydb "github.com/google/exposure-notifications-server/internal/verification/database"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/mikehelmick/go-chaff"
)

const (
	HeaderAPIVersion = "x-api-version"
)

type Server struct {
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

func NewServer(ctx context.Context, cfg *Config, env *serverenv.ServerEnv) (*Server, error) {
	logger := logging.FromContext(ctx).Named("publish")

	logger.Debugw("creating server",
		"max_keys_on_publish", cfg.MaxKeysOnPublish,
		"max_same_start_interval_keys", cfg.MaxSameStartIntervalKeys,
		"max_interval_age", cfg.MaxIntervalAge,
		"truncate_window", cfg.TruncateWindow)
	if cfg.DebugReleaseSameDayKeys() {
		logger.Warnw("SERVER IS IN DEBUG MODE - KEYS MAY BE RELEASED EARLY!")
	}

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.AuthorizedAppProvider() == nil {
		return nil, fmt.Errorf("missing AuthorizedApp provider in server environment")
	}

	transformer, err := model.NewTransformer(cfg)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}

	verifier, err := verification.New(verifydb.New(env.Database()), &cfg.Verification)
	if err != nil {
		return nil, fmt.Errorf("verification.New: %w", err)
	}

	aadBytes := cfg.RevisionToken.AAD
	if len(aadBytes) == 0 {
		return nil, fmt.Errorf("must provide Additional Authenticated Data (AAD) for revision token encryption in REVISION_TOKEN_AAD env variable")
	}
	revisionKeyConfig := revisiondb.KMSConfig{
		WrapperKeyID: cfg.RevisionToken.KeyID,
		KeyManager:   env.GetKeyManager(),
	}
	revisionDB, err := revisiondb.New(env.Database(), &revisionKeyConfig)
	if err != nil {
		return nil, fmt.Errorf("revisiondb.New: %w", err)
	}
	tm, err := revision.New(ctx, revisionDB, cfg.RevisionKeyCacheDuration, cfg.RevisionToken.MinLength)
	if err != nil {
		return nil, fmt.Errorf("revision.New: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	chaffer, err := chaff.NewTracker(chaff.NewJSONResponder(chaffPublishResponse), chaff.DefaultCapacity)
	if err != nil {
		return nil, fmt.Errorf("error making chaffer: %w", err)
	}

	return &Server{
		serverenv:             env,
		transformer:           transformer,
		config:                cfg,
		database:              database.New(env.Database()),
		tracker:               chaffer,
		tokenManager:          tm,
		tokenAAD:              aadBytes,
		authorizedAppProvider: env.AuthorizedAppProvider(),
		verifier:              verifier,
	}, nil
}

type response struct {
	status      int
	pubResponse *verifyapi.PublishResponse

	// metrics is a function to execute to publish metrics
	metrics func()
}

func generatePadding(minPadding, paddingRange int64) (string, error) {
	minBytes := minPadding
	if minBytes <= 0 {
		minBytes = 1024
	}
	padRange := int64(paddingRange)
	if padRange <= 0 {
		padRange = 1024
	}

	bi, err := rand.Int(rand.Reader, big.NewInt(padRange))
	if err != nil {
		return "", fmt.Errorf("padding: failed to generate random number: %w", err)
	}
	i := int(bi.Int64() + minBytes)

	b := make([]byte, i)
	n, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("padding: failed to read bytes: %w", err)
	}
	if n < i {
		return "", fmt.Errorf("padding: wrote less bytes than expected")
	}

	return base64.StdEncoding.EncodeToString(b), nil
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
func (s *Server) process(ctx context.Context, data *verifyapi.Publish, platform string, bridge *versionBridge) *response {
	ctx, span := trace.StartSpan(ctx, "(*publish.PublishHandler).process")
	defer span.End()

	logger := logging.FromContext(ctx).With("health_authority_id", data.HealthAuthorityID)

	logger.Info("publish API request")

	appConfig, err := s.authorizedAppProvider.AppConfig(ctx, data.HealthAuthorityID)
	if err != nil {
		// Config loaded, but app with that name isn't registered. This can also
		// happen if the app was recently registered but the cache hasn't been
		// refreshed.
		if errors.Is(err, authorizedapp.ErrAppNotFound) {
			message := fmt.Sprintf("unauthorized health authority: %v", data.HealthAuthorityID)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return &response{
				status: http.StatusUnauthorized,
				pubResponse: &verifyapi.PublishResponse{
					ErrorMessage: message,
					Code:         verifyapi.ErrorUnknownHealthAuthorityID,
				},
				metrics: func() {
					stats.Record(ctx, mHealthAuthorityNotAuthorized.M(1))
				},
			}
		}

		// A higher-level configuration error occurred, likely while trying to read
		// from the database. This is retryable, although won't succeed if the error
		// isn't transient.
		// This message (and logging) will only contain the AppPkgName from the request
		// and no other data from the request.
		message := fmt.Sprintf("error loading health authority config: %v", err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return &response{
			status: http.StatusNotFound,
			pubResponse: &verifyapi.PublishResponse{
				ErrorMessage: message,
				Code:         verifyapi.ErrorUnableToLoadHealthAuthority,
			},
			metrics: func() {
				stats.Record(ctx, mErrorLoadingAuthorizedApp.M(1))
			},
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
				return &response{
					status: http.StatusUnauthorized,
					pubResponse: &verifyapi.PublishResponse{
						ErrorMessage: message, // Error code omitted, since this isn't in the v1 path.
					},
					metrics: func() {
						stats.Record(ctx, mRegionNotAuthorized.M(1))
					},
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
	if len(regions) == 0 && s.config.DefaultRegion != "" {
		regions = append(regions, s.config.DefaultRegion)
	}

	// Verify that there is at least one region set by API call or by one of the
	// generous defaults. If there isn't a region set, then the TEKs
	// won't be retrievable, so we ensure there is something set.
	if len(regions) == 0 {
		logger.Errorf("No regions present in request or configured for HealthAuthorityID: %v", data.HealthAuthorityID)
		message := fmt.Sprintf("unknown health authority regions for %v", data.HealthAuthorityID)
		span.SetStatus(trace.Status{Code: trace.StatusCodePermissionDenied, Message: message})
		return &response{
			status: http.StatusInternalServerError,
			pubResponse: &verifyapi.PublishResponse{
				ErrorMessage: message,
				Code:         verifyapi.ErrorHealthAuthorityMissingRegionConfiguration,
			},
			metrics: func() {
				stats.Record(ctx, mRegionNotSpecified.M(1))
			},
		}
	}

	// Perform health authority certificate verification.
	verifiedClaims, err := s.verifier.VerifyDiagnosisCertificate(ctx, appConfig, data)
	if err != nil {
		if appConfig.BypassHealthAuthorityVerification {
			logger.Warnf("bypassing health authority certificate verification health authority: %v", appConfig.AppPackageName)
			stats.Record(ctx, mVerificationBypassed.M(1))
		} else {
			message := fmt.Sprintf("unable to validate diagnosis verification: %v", err)
			if s.config.DebugLogBadCertificates {
				logger.Errorw(message, "error", err, "jwt", data.VerificationPayload)
			} else {
				logger.Errorw(message, "error", err)
			}
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
			return &response{
				status: http.StatusUnauthorized,
				pubResponse: &verifyapi.PublishResponse{
					ErrorMessage: message,
					Code:         verifyapi.ErrorVerificationCertificateInvalid,
				},
				metrics: func() {
					stats.Record(ctx, mBadVerification.M(1))
				},
			}
		}
	}

	// Examine the revision token. It is expected that it is missing in most cases.
	var token *pb.RevisionTokenData
	decryptFail := false
	if len(data.RevisionToken) != 0 {
		encryptedToken, err := base64util.DecodeString(data.RevisionToken)
		if err != nil {
			logger.Errorf("unable to decode revision token, proceeding without: %v", err)
		} else {
			token, err = s.tokenManager.UnmarshalRevisionToken(ctx, encryptedToken, s.tokenAAD)
			if err != nil {
				logger.Errorf("unable to unmarshall / decrypt revision token. Treating as if none was provided: %v", err)
				token = nil // just in case.
				decryptFail = true
			}
		}
	}

	batchTime := time.Now()
	result, transformError := s.transformer.TransformPublish(ctx, data, regions, verifiedClaims, batchTime)
	// Break apart the result object for easier usage below.
	exposures := result.Exposures
	publishInfo := result.PublishInfo
	transformWarnings := result.Warnings
	// Check for non-recoverable error. It is possible that individual keys are dropped, but if there
	// are any valid ones, we will try and move forward.
	// If at the end, there is a success, the transformError will be returned as supplemental information.
	if transformError != nil && len(exposures) == 0 {
		message := fmt.Sprintf("unable to read request data: %v", transformError)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
		return &response{
			status: http.StatusBadRequest,
			pubResponse: &verifyapi.PublishResponse{
				ErrorMessage: message,
				Code:         verifyapi.ErrorBadRequest,
				Warnings:     transformWarnings,
			},
			metrics: func() {
				stats.Record(ctx, mTransformFail.M(1))
			},
		}
	}

	// Add in the platform
	if publishInfo != nil {
		publishInfo.Platform = platform
	}

	resp, err := s.database.InsertAndReviseExposures(ctx, &database.InsertAndReviseExposuresRequest{
		Incoming:    exposures,
		Token:       token,
		PublishInfo: publishInfo,

		RequireToken:          !appConfig.BypassRevisionToken,
		AllowPartialRevisions: s.config.AllowPartialRevisions,
	})
	if err != nil {
		status := http.StatusBadRequest
		var logMessage, errorMessage, errorCode string
		var errInvalidReportTypeTransition *model.ErrorKeyInvalidReportTypeTransition
		metric := "publish-revision-token-issue"
		switch {
		case decryptFail || errors.Is(err, database.ErrExistingKeyNotInToken) || errors.Is(err, database.ErrRevisionTokenMetadataMismatch):
			logMessage = fmt.Sprintf("revision token present, but invalid: %v", err)
			errorMessage = "revision token is invalid"
			errorCode = verifyapi.ErrorInvalidRevisionToken
		case errors.Is(err, database.ErrNoRevisionToken):
			logMessage = "no revision token"
			errorMessage = "no revision token, but sent existing keys"
			errorCode = verifyapi.ErrorMissingRevisionToken
		case errors.Is(err, model.ErrorKeyAlreadyRevised):
			logMessage = "key already revised"
			errorMessage = "key was already revised"
			errorCode = verifyapi.ErrorKeyAlreadyRevised
		case errors.As(err, &errInvalidReportTypeTransition):
			logMessage = errInvalidReportTypeTransition.Error()
			errorMessage = errInvalidReportTypeTransition.Error()
			errorCode = verifyapi.ErrorInvalidReportTypeTransition
		default:
			logMessage = fmt.Sprintf("error writing exposure record: %v", err)
			errorMessage = http.StatusText(http.StatusInternalServerError)
			errorCode = verifyapi.ErrorInternalError
			metric = "publish-db-write-error"
			logger.Errorw("publish error", "error", logMessage)
		}
		logger.Debugw("publish error", "error", logMessage)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: logMessage})
		return &response{
			status: status,
			pubResponse: &verifyapi.PublishResponse{
				ErrorMessage: errorMessage,
				Code:         errorCode,
				Warnings:     transformWarnings,
			},
			metrics: func() {
				// TODO(yegle): we want to use this string later when we
				// refactor the metrics.
				_ = metric
				stats.Record(ctx, mRevisionTokenIssue.M(1))
			},
		}
	}

	// Build the new revision token. Union of existing token take + new exposures.
	var keep pb.RevisionTokenData
	if token != nil {
		// put existing tokens that aren't too old back in the token.
		retainInterval := model.IntervalNumber(batchTime.Add(-1 * s.config.MaxIntervalAge))
		for _, rk := range token.RevisableKeys {
			if rk.IntervalNumber+rk.IntervalCount >= retainInterval {
				keep.RevisableKeys = append(keep.RevisableKeys, rk)
			}
		}
	}
	newToken, err := s.tokenManager.MakeRevisionToken(ctx, &keep, resp.Exposures, s.tokenAAD)
	if err != nil {
		// In this case, an empty revision token is returned. The rest of the request
		// was handled correctly.
		logger.Errorf("unable to make new revision token: %v", err)
		newToken = make([]byte, 0)
	}

	span.AddAttributes(trace.Int64Attribute("exposures_inserted", int64(resp.Inserted)))
	span.AddAttributes(trace.Int64Attribute("exposures_revised", int64(resp.Revised)))
	span.AddAttributes(trace.Int64Attribute("exposures_dropped", int64(resp.Dropped)))
	logger.Infow("published exposures",
		"inserted", resp.Inserted,
		"updated", resp.Revised,
		"dropped", resp.Dropped)

	publishResponse := verifyapi.PublishResponse{
		RevisionToken:     base64.StdEncoding.EncodeToString(newToken),
		InsertedExposures: int(resp.Inserted),
		Warnings:          transformWarnings,
	}
	// If there was a partial failure on transform, add that information back into the success response.
	if transformError != nil {
		publishResponse.Code = verifyapi.ErrorPartialFailure
		publishResponse.ErrorMessage = transformError.Error()
	}

	return &response{
		status:      http.StatusOK,
		pubResponse: &publishResponse,
		metrics: func() {
			stats.Record(ctx, mExposuresInserted.M(int64(resp.Inserted)))
			stats.Record(ctx, mExposuresInserted.M(int64(resp.Revised)))
			stats.Record(ctx, mExposuresInserted.M(int64(resp.Dropped)))
		},
	}
}

// chaffPushResponse takes a chaffing string, and builds a chaff response.
func chaffPublishResponse(s string) interface{} {
	return verifyapi.PublishResponse{Padding: s}
}
