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

// Package federationout handles requests from other federation servers for data.
package federationout

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/federationin/model"
	"github.com/google/exposure-notifications-server/internal/pb/federation"
	"github.com/google/exposure-notifications-server/pkg/logging"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/federationout/database"
	"github.com/google/exposure-notifications-server/internal/metrics/metricsware"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"

	"github.com/google/exposure-notifications-server/internal/serverenv"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authHeader = "authorization"
	bearer     = "Bearer"
)

// Compile time assert that this server implements the required grpc interface.
var _ federation.FederationServer = (*Server)(nil)

type iterateExposuresFunc func(context.Context, publishdb.IterateExposuresCriteria, publishdb.IteratorFunction) (string, error)

// NewServer builds a new FederationServer.
func NewServer(env *serverenv.ServerEnv, config *Config) federation.FederationServer {
	return &Server{
		env:       env,
		db:        database.New(env.Database()),
		publishdb: publishdb.New(env.Database()),
		config:    config,
	}
}

type Server struct {
	env       *serverenv.ServerEnv
	db        *database.FederationOutDB
	publishdb *publishdb.PublishDB
	config    *Config
}

type authKey struct{}

// Fetch implements the FederationServer Fetch endpoint.
func (s Server) Fetch(ctx context.Context, req *federation.FederationFetchRequest) (*federation.FederationFetchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	logger := logging.FromContext(ctx)
	response, err := s.fetch(ctx, req, s.publishdb.IterateExposures, publishmodel.TruncateWindow(time.Now(), s.config.TruncateWindow)) // Don't fetch the current window, which isn't complete yet. TODO(squee1945): should I double this for safety?
	if err != nil {
		metricsExporter := s.env.MetricsExporter(ctx)
		metricsMiddleWare := metricsware.NewMiddleWare(&metricsExporter)
		metricsMiddleWare.RecordFetchFailure(ctx)
		logger.Errorf("Fetch error: %v", err)
		return nil, errors.New("internal error")
	}
	return response, nil
}

func (s Server) fetch(ctx context.Context, req *federation.FederationFetchRequest, itFunc iterateExposuresFunc, fetchUntil time.Time) (*federation.FederationFetchResponse, error) {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)
	metricsMiddleWare := metricsware.NewMiddleWare(&metrics)

	req.Region = strings.ToUpper(req.Region)
	metricsMiddleWare.RecordFetchRegionsRequested(ctx, 1)

	logger.Infof("Processing client request %#v", req)

	// If there is a FederationAuthorization on the context, set the query to operate within its limits.
	if auth, ok := ctx.Value(authKey{}).(*model.FederationOutAuthorization); ok {
		if !contains(auth.IncludeRegions, req.Region) {
			return nil, fmt.Errorf("unauthorized region requested")
		}
	}

	state := req.GetState()
	if state == nil {
		state = &federation.FetchState{
			KeyCursor:        &federation.Cursor{},
			RevisedKeyCursor: &federation.Cursor{},
		}
	}

	// Primary (non-revised) keys are read first.
	criteria := publishdb.IterateExposuresCriteria{
		IncludeRegions:      []string{req.Region},
		SinceTimestamp:      time.Unix(state.KeyCursor.Timestamp, 0),
		UntilTimestamp:      fetchUntil,
		LastCursor:          state.KeyCursor.NextToken,
		IncludeTravelers:    req.IncludeTravelers,
		OnlyTravelers:       req.OnlyTravelers,
		OnlyLocalProvenance: req.OnlyLocalProvenance, // Include re-federation?
	}
	state.KeyCursor.NextToken = ""

	logger.Infof("Query criteria: %#v", criteria)

	response := &federation.FederationFetchResponse{
		Keys:           []*federation.ExposureKey{},
		RevisedKeys:    []*federation.ExposureKey{},
		NextFetchState: state,
	}
	count := 0
	cursor, err := itFunc(ctx, criteria, buildIteratorFunction(&BuildIteratorRequest{
		destination: response.Keys,
		revised:     false,
		state:       state,
		count:       &count,
	}))
	keepGoing := true
	if err != nil {
		metricsMiddleWare.RecordFetchError(ctx)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logger.Infof("Fetch request reached time out, returning partial response.")
			response.PartialResponse = true
			state.KeyCursor.NextToken = cursor
			keepGoing = false
		} else {
			return nil, err
		}
	}

	if keepGoing {
		criteria.OnlyRevisedKeys = true
		criteria.SinceTimestamp = time.Unix(state.RevisedKeyCursor.Timestamp, 0)
		criteria.LastCursor = state.RevisedKeyCursor.NextToken
		state.RevisedKeyCursor.NextToken = ""

		cursor, err := itFunc(ctx, criteria, buildIteratorFunction(&BuildIteratorRequest{
			destination: response.RevisedKeys,
			revised:     true,
			state:       state,
			count:       &count,
		}))
		if err != nil {
			metricsMiddleWare.RecordFetchError(ctx)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				logger.Infof("Fetch request reached time out, returning partial response.")
				response.PartialResponse = true
				state.RevisedKeyCursor.NextToken = cursor
			} else {
				return nil, err
			}
		}
	}

	metricsMiddleWare.RecordFetchCount(ctx, count)
	logger.Infof("Sent %d keys", count)
	return response, nil
}

type BuildIteratorRequest struct {
	destination []*federation.ExposureKey
	revised     bool
	state       *federation.FetchState
	count       *int
}

func reportType(reportType string) federation.ExposureKey_ReportType {
	switch reportType {
	case verifyapi.ReportTypeConfirmed:
		return federation.ExposureKey_CONFIRMED_TEST
	case verifyapi.ReportTypeClinical:
		return federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS
	case verifyapi.ReportTypeNegative:
		return federation.ExposureKey_REVOKED
	default:
		return federation.ExposureKey_UNKNOWN
	}
}

func buildIteratorFunction(request *BuildIteratorRequest) publishdb.IteratorFunction {
	return func(exp *publishmodel.Exposure) error {
		// If the diagnosis key is empty, it's malformed, so skip it.
		if len(exp.ExposureKey) != 16 {
			return nil
		}

		key := federation.ExposureKey{
			ExposureKey:    exp.ExposureKey,
			IntervalNumber: exp.IntervalNumber,
			IntervalCount:  exp.IntervalCount,
		}

		if !request.revised {
			// primary keys first
			key.TransmissionRisk = int32(exp.TransmissionRisk)
			key.ReportType = reportType(exp.ReportType)
			if exp.HasDaysSinceSymptomOnset() {
				key.DaysSinceOnsetOfSymptoms = *exp.DaysSinceSymptomOnset
			}

			created := exp.CreatedAt
			if ts := created.Unix(); ts > request.state.KeyCursor.Timestamp {
				request.state.KeyCursor.Timestamp = ts
			}
		} else {
			// Revised keys get different fields
			key.TransmissionRisk = int32(*exp.RevisedTransmissionRisk)
			key.ReportType = reportType(*exp.RevisedReportType)
			if exp.RevisedDaysSinceSymptomOnset != nil {
				key.DaysSinceOnsetOfSymptoms = *exp.RevisedDaysSinceSymptomOnset
			}

			revisedAt := *exp.RevisedAt
			if ts := revisedAt.Unix(); ts > request.state.RevisedKeyCursor.Timestamp {
				request.state.RevisedKeyCursor.Timestamp = ts
			}
		}

		request.destination = append(request.destination, &key)

		*request.count++
		return nil
	}
}

// AuthInterceptor validates incoming OIDC bearer token and adds corresponding FederationAuthorization record to the context.
func (s Server) AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)
	metricsMiddleware := metricsware.NewMiddleWare(&metrics)

	raw, err := rawToken(ctx)
	if err != nil {
		logger.Infof("Invalid headers: %v", err)
		return nil, err
	}

	token, err := idtoken.Validate(ctx, raw, "")
	if err != nil {
		logger.Infof("Invalid token: %v", err)
		metricsMiddleware.RecordInvalidFetchAuthToken(ctx)
		return nil, status.Errorf(codes.Unauthenticated, "Invalid token")
	}

	auth, err := s.db.GetFederationOutAuthorization(ctx, token.Issuer, token.Subject)
	if err != nil {
		if errors.Is(err, coredb.ErrNotFound) {
			metricsMiddleware.RecordUnauthorizedFetchAttempt(ctx)
			logger.Infof("Authorization not found (issuer %q, subject %s)", token.Issuer, token.Subject)
			return nil, status.Errorf(codes.Unauthenticated, "Invalid issuer/subject")
		}
		logger.Errorf("Failed to fetch authorization (issuer %q, subject %s): %v", token.Issuer, token.Subject, err)
		metricsMiddleware.RecordInternalErrorDuringFetch(ctx)
		return nil, status.Errorf(codes.Internal, "Internal error")
	}

	if auth.Audience != "" && auth.Audience != token.Audience {
		metricsMiddleware.RecordInvalidAudienceDuringFetch(ctx)
		logger.Infof("Invalid audience, got %q, want %q", token.Audience, auth.Audience)
		return nil, status.Errorf(codes.Unauthenticated, "Invalid audience")
	}

	// Store the FederationAuthorization on the context.
	logger.Infof("Caller: issuer %q subject %q", auth.Issuer, auth.Subject)
	ctx = context.WithValue(ctx, authKey{}, auth)
	return handler(ctx, req)
}

func rawToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "Missing metadata")
	}
	if _, ok := md[authHeader]; !ok {
		return "", status.Errorf(codes.Unauthenticated, "Missing authorization header [1]")
	}
	if len(md[authHeader]) == 0 {
		return "", status.Errorf(codes.Unauthenticated, "Missing authorization header [2]")
	}
	if len(md[authHeader]) > 1 {
		return "", status.Errorf(codes.Unauthenticated, "Multiple authorization headers")
	}

	authHeader := md[authHeader][0]
	if !strings.HasPrefix(authHeader, bearer) {
		return "", status.Errorf(codes.Unauthenticated, "Invalid authorization header")
	}
	rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, bearer))
	return rawToken, nil
}

func contains(arr []string, target string) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}
