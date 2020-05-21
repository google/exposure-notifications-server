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
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/pb"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
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
var _ pb.FederationServer = (*Server)(nil)

type iterateExposuresFunc func(context.Context, database.IterateExposuresCriteria, func(*model.Exposure) error) (string, error)

// NewServer builds a new FederationServer.
func NewServer(env *serverenv.ServerEnv, config *Config) pb.FederationServer {
	return &Server{
		env:        env,
		db:         env.Database(),
		exposuredb: database.New(env.Database()),
		config:     config,
	}
}

type Server struct {
	env        *serverenv.ServerEnv
	db         *coredb.DB
	exposuredb *database.ExposureDB
	config     *Config
}

type authKey struct{}

// Fetch implements the FederationServer Fetch endpoint.
func (s Server) Fetch(ctx context.Context, req *pb.FederationFetchRequest) (*pb.FederationFetchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	logger := logging.FromContext(ctx)
	response, err := s.fetch(ctx, req, s.exposuredb.IterateExposures, model.TruncateWindow(time.Now(), s.config.TruncateWindow)) // Don't fetch the current window, which isn't complete yet. TODO(squee1945): should I double this for safety?
	if err != nil {
		s.env.MetricsExporter(ctx).WriteInt("federation-fetch-failed", true, 1)
		logger.Errorf("Fetch error: %v", err)
		return nil, errors.New("internal error")
	}
	return response, nil
}

func (s Server) fetch(ctx context.Context, req *pb.FederationFetchRequest, itFunc iterateExposuresFunc, fetchUntil time.Time) (*pb.FederationFetchResponse, error) {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)

	for i := range req.RegionIdentifiers {
		req.RegionIdentifiers[i] = strings.ToUpper(req.RegionIdentifiers[i])
	}
	for i := range req.ExcludeRegionIdentifiers {
		req.ExcludeRegionIdentifiers[i] = strings.ToUpper(req.ExcludeRegionIdentifiers[i])
	}
	metrics.WriteInt("federation-fetch-regions-requested", false, len(req.RegionIdentifiers))
	metrics.WriteInt("federation-fetch-regions-excluded", false, len(req.ExcludeRegionIdentifiers))

	logger.Infof("Processing client request %#v", req)

	// If there is a FederationAuthorization on the context, set the query to operate within its limits.
	if auth, ok := ctx.Value(authKey{}).(*coredb.FederationOutAuthorization); ok {
		// For included regions, we INTERSECT the requested included regions with the configured included regions.
		req.RegionIdentifiers = intersect(req.RegionIdentifiers, auth.IncludeRegions)
		// For excluded regions, we UNION the the requested excluded regions with the configured excluded regions.
		req.ExcludeRegionIdentifiers = union(req.ExcludeRegionIdentifiers, auth.ExcludeRegions)
	}

	criteria := database.IterateExposuresCriteria{
		IncludeRegions:      req.RegionIdentifiers,
		ExcludeRegions:      req.ExcludeRegionIdentifiers,
		SinceTimestamp:      time.Unix(req.LastFetchResponseKeyTimestamp, 0),
		UntilTimestamp:      fetchUntil,
		LastCursor:          req.NextFetchToken,
		OnlyLocalProvenance: true, // Do not return results that came from other federation partners.
	}

	logger.Infof("Query criteria: %#v", criteria)

	// Filter included countries in memory.
	includedRegions := map[string]struct{}{}
	for _, region := range req.RegionIdentifiers {
		includedRegions[region] = struct{}{}
	}

	// Filter excluded countries in memory, using a map for efficiency.
	excludedRegions := map[string]struct{}{}
	for _, region := range req.ExcludeRegionIdentifiers {
		excludedRegions[region] = struct{}{}
	}

	ctrMap := map[string]*pb.ContactTracingResponse{} // local index into the response being assembled; keyed on unique set of regions.
	ctiMap := map[string]*pb.ContactTracingInfo{}     // local index into the response being assembled; keys on unique set of (ctrMap key, transmissionRisk, verificationAuthorityName)
	response := &pb.FederationFetchResponse{}
	count := 0
	cursor, err := itFunc(ctx, criteria, func(inf *model.Exposure) error {
		// If the diagnosis key is empty, it's malformed, so skip it.
		if len(inf.ExposureKey) == 0 {
			logger.Debugf("Exposure %s missing ExposureKey, skipping.", inf.ExposureKey)
			return nil
		}

		// If there are no regions on the exposure, it's malformed, so skip it.
		if len(inf.Regions) == 0 {
			logger.Debugf("Exposure %s missing Regions, skipping.", inf.ExposureKey)
			return nil
		}

		// Filter out non-LocalProvenance results; we should not re-federate.
		// This may already be handled by the database query and is included here for completeness.
		if !inf.LocalProvenance {
			logger.Debugf("Exposure %s not LocalProvenance, skipping.", inf.ExposureKey)
			return nil
		}

		// If all the regions on the record are excluded, skip it.
		skip := true
		for _, region := range inf.Regions {
			if _, excluded := excludedRegions[region]; !excluded {
				// At least one region for the exposure is NOT excluded, so we don't skip this record.
				skip = false
				break
			}
		}
		if skip {
			logger.Debugf("Exposure %s contains only excluded regions, skipping.", inf.ExposureKey)
			return nil
		}

		// If filtering on a region (len(includedRegions) > 0) and none of the regions on the record are included, skip it.
		if len(includedRegions) > 0 {
			skip = true
			for _, region := range inf.Regions {
				if _, included := includedRegions[region]; included {
					skip = false
					break
				}
			}
			if skip {
				logger.Debugf("Exposure %s does not contain requested regions, skipping.", inf.ExposureKey)
				return nil
			}
		}

		// Find, or create, the ContactTracingResponse based on the unique set of regions.
		sort.Strings(inf.Regions)
		ctrKey := strings.Join(inf.Regions, "::")
		ctr := ctrMap[ctrKey]
		if ctr == nil {
			ctr = &pb.ContactTracingResponse{RegionIdentifiers: inf.Regions}
			ctrMap[ctrKey] = ctr
			response.Response = append(response.Response, ctr)
		}

		// Find, or create, the ContactTracingInfo for (ctrKey, transmissionRisk).
		ctiKey := fmt.Sprintf("%s::%d", ctrKey, inf.TransmissionRisk)
		cti := ctiMap[ctiKey]
		if cti == nil {
			cti = &pb.ContactTracingInfo{TransmissionRisk: int32(inf.TransmissionRisk)}
			ctiMap[ctiKey] = cti
			ctr.ContactTracingInfo = append(ctr.ContactTracingInfo, cti)
		}

		// Add the key to the ContactTracingInfo.
		cti.ExposureKeys = append(cti.ExposureKeys, &pb.ExposureKey{
			ExposureKey:    inf.ExposureKey,
			IntervalNumber: inf.IntervalNumber,
			IntervalCount:  inf.IntervalCount,
		})

		created := inf.CreatedAt.Unix()
		if created > response.FetchResponseKeyTimestamp {
			response.FetchResponseKeyTimestamp = created
		}

		count++
		return nil
	})
	if err != nil {
		metrics.WriteInt("federation-fetch-error", true, 1)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logger.Infof("Fetch request reached time out, returning partial response.")
			response.PartialResponse = true
			response.NextFetchToken = cursor
		} else {
			return nil, err
		}
	}
	metrics.WriteInt("federation-fetch-count", false, count)
	logger.Infof("Sent %d keys", count)
	return response, nil
}

// AuthInterceptor validates incoming OIDC bearer token and adds corresponding FederationAuthorization record to the context.
func (s Server) AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)

	raw, err := rawToken(ctx)
	if err != nil {
		logger.Infof("Invalid headers: %v", err)
		return nil, err
	}

	token, err := idtoken.Validate(ctx, raw, "")
	if err != nil {
		logger.Infof("Invalid token: %v", err)
		metrics.WriteInt("federation-fetch-invalid-auth-token", true, 1)
		return nil, status.Errorf(codes.Unauthenticated, "Invalid token")
	}

	auth, err := s.db.GetFederationOutAuthorization(ctx, token.Issuer, token.Subject)
	if err != nil {
		if errors.Is(err, coredb.ErrNotFound) {
			metrics.WriteInt("federation-fetch-unauthorized", true, 1)
			logger.Infof("Authorization not found (issuer %q, subject %s)", token.Issuer, token.Subject)
			return nil, status.Errorf(codes.Unauthenticated, "Invalid issuer/subject")
		}
		logger.Errorf("Failed to fetch authorization (issuer %q, subject %s): %v", token.Issuer, token.Subject, err)
		metrics.WriteInt("federation-fetch-internal-error", true, 1)
		return nil, status.Errorf(codes.Internal, "Internal error")
	}

	if auth.Audience != "" && auth.Audience != token.Audience {
		metrics.WriteInt("federation-fetch-invalid-audience", true, 1)
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

func intersect(aa, bb []string) []string {
	if len(aa) == 0 || len(bb) == 0 {
		return nil
	}
	var result []string
	for _, a := range aa {
		for _, b := range bb {
			if a == b {
				result = append(result, a)
				break
			}
		}
	}
	return result
}

func union(aa, bb []string) []string {
	if len(aa) == 0 {
		return bb
	}
	if len(bb) == 0 {
		return aa
	}
	m := map[string]struct{}{}
	for _, a := range aa {
		m[a] = struct{}{}
	}
	for _, b := range bb {
		m[b] = struct{}{}
	}
	var result []string
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
