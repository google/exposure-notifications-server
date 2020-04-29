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

package api

import (
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// type diagKeyList []*pb.ExposureKey
// type diagKeys map[pb.TransmissionRisk]diagKeyList
// type collator map[string]diagKeys
type fetchIterator func(context.Context, database.IterateInfectionsCriteria) (database.InfectionIterator, error)

// NewFederationServer builds a new FederationServer.
func NewFederationServer(timeout time.Duration) pb.FederationServer {
	return &federationServer{timeout: timeout}
}

type federationServer struct {
	timeout time.Duration
}

// Fetch implements the FederationServer Fetch endpoint.
func (s *federationServer) Fetch(ctx context.Context, req *pb.FederationFetchRequest) (*pb.FederationFetchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	logger := logging.FromContext(ctx)
	response, err := s.fetch(ctx, req, database.IterateInfections, model.TruncateWindow(time.Now().UTC())) // Don't fetch the current window, which isn't complete yet. TODO(jasonco): should I double this for safety?
	if err != nil {
		logger.Errorf("Fetch error: %v", err)
		return nil, errors.New("internal error")
	}
	return response, nil
}

func (s *federationServer) fetch(ctx context.Context, req *pb.FederationFetchRequest, itFunc fetchIterator, fetchUntil time.Time) (*pb.FederationFetchResponse, error) {
	logger := logging.FromContext(ctx)

	for i := range req.RegionIdentifiers {
		req.RegionIdentifiers[i] = strings.ToUpper(req.RegionIdentifiers[i])
	}
	for i := range req.ExcludeRegionIdentifiers {
		req.ExcludeRegionIdentifiers[i] = strings.ToUpper(req.ExcludeRegionIdentifiers[i])
	}

	// If there is only one region, we can let datastore filter it; otherwise we'll have to filter in memory.
	// TODO(jasonco): Filter out other partner's data; don't re-federate.
	// TODO(jasonco): moving to CloudSQL will allow this to be simplified.
	criteria := database.IterateInfectionsCriteria{
		SinceTimestamp:      time.Unix(req.LastFetchResponseKeyTimestamp, 0).UTC(),
		UntilTimestamp:      fetchUntil,
		LastCursor:          req.NextFetchToken,
		OnlyLocalProvenance: true, // Do not return results that came from other federation partners.
	}
	if len(req.RegionIdentifiers) == 1 {
		criteria.IncludeRegions = req.RegionIdentifiers
	}

	logger.Infof("Processing request Regions:%v Excluding:%v Since:%v Until:%v HasCursor:%t", req.RegionIdentifiers, req.ExcludeRegionIdentifiers, criteria.SinceTimestamp, criteria.UntilTimestamp, req.NextFetchToken != "")

	// Filter included countries in memory.
	// TODO(jasonco): move to database query if/when Cloud SQL.
	includedRegions := map[string]struct{}{}
	for _, region := range req.RegionIdentifiers {
		includedRegions[region] = struct{}{}
	}

	// Filter excluded countries in memory, using a map for efficiency.
	// TODO(jasonco): move to database query if/when Cloud SQL.
	excludedRegions := map[string]struct{}{}
	for _, region := range req.ExcludeRegionIdentifiers {
		excludedRegions[region] = struct{}{}
	}

	it, err := itFunc(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("querying infections (criteria: %#v): %v", criteria, err)
	}
	defer it.Close()

	ctrMap := map[string]*pb.ContactTracingResponse{} // local index into the response being assembled; keyed on unique set of regions.
	ctiMap := map[string]*pb.ContactTracingInfo{}     // local index into the response being assembled; keys on unique set of (ctrMap key, transmissionRisk, verificationAuthorityName)
	response := &pb.FederationFetchResponse{}

	count := 0
	for !response.PartialResponse { // This loop will end on break, or if the context is interrupted and we send a partial response.

		// Check the context to see if we've been interrupted (e.g., timeout).
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled { // May be context.Canceled due to test code.
				return nil, fmt.Errorf("context error: %v", err)
			}

			cursor, err := it.Cursor()
			if err != nil {
				return nil, fmt.Errorf("generating cursor: %v", err)
			}

			logger.Infof("Fetch request reached time out, returning partial response.")
			response.PartialResponse = true
			response.NextFetchToken = cursor
			continue

		default:
			// Fallthrough to process a record.
		}

		inf, done, err := it.Next()
		if err != nil {
			return nil, fmt.Errorf("iterating results: %v", err)
		}

		if done {
			// Reached the end of the result set.
			break
		}
		if inf == nil {
			continue
		}

		// If the diagnosis key is empty, it's malformed, so skip it.
		if len(inf.ExposureKey) == 0 {
			logger.Debugf("Infection %s missing ExposureKey, skipping.", inf.ExposureKey)
			continue
		}

		// If there are no regions on the infection, it's malformed, so skip it.
		if len(inf.Regions) == 0 {
			logger.Debugf("Infection %s missing Regions, skipping.", inf.ExposureKey)
			continue
		}

		// Filter out non-LocalProvenance results; we should not re-federate.
		// This may already be handled by the database query and is included here for completeness.
		if !inf.LocalProvenance {
			logger.Debugf("Infection %s not LocalProvenance, skipping.", inf.ExposureKey)
			continue
		}

		// If the infection has an unknown status, it's malformed, so skip it.
		if _, ok := pb.TransmissionRisk_name[int32(inf.TransmissionRisk)]; !ok {
			logger.Debugf("Infection %s has invalid TransmissionRisk, skipping.", inf.ExposureKey)
			continue
		}

		// If all the regions on the record are excluded, skip it.
		// TODO(jasonco): move to database query if/when Cloud SQL.
		skip := true
		for _, region := range inf.Regions {
			if _, excluded := excludedRegions[region]; !excluded {
				// At least one region for the infection is NOT excluded, so we don't skip this record.
				skip = false
				break
			}
		}
		if skip {
			logger.Debugf("Infection %s contains only excluded regions, skipping.", inf.ExposureKey)
			continue
		}

		// If filtering on a region (len(includedRegions) > 0) and none of the regions on the record are included, skip it.
		// TODO(jasonco): move to database query if/when Cloud SQL.
		if len(includedRegions) > 0 {
			skip = true
			for _, region := range inf.Regions {
				if _, included := includedRegions[region]; included {
					skip = false
					break
				}
			}
			if skip {
				logger.Debugf("Infection %s does not contain requested regions, skipping.", inf.ExposureKey)
				continue
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

		// Find, or create, the ContactTracingInfo for (ctrKey, transmissionRisk, verificationAuthorityName).
		status := pb.TransmissionRisk(inf.TransmissionRisk)
		ctiKey := fmt.Sprintf("%s::%d::%s", ctrKey, status, inf.VerificationAuthorityName)
		cti := ctiMap[ctiKey]
		if cti == nil {
			cti = &pb.ContactTracingInfo{TransmissionRisk: status, VerificationAuthorityName: inf.VerificationAuthorityName}
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
	}

	logger.Infof("Sent %d keys", count)
	return response, nil
}
