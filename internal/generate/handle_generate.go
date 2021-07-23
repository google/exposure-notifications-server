// Copyright 2021 the Exposure Notifications Server authors
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

package generate

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/pb"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-server/pkg/util"
	"go.opencensus.io/stats"
)

func (s *Server) handleGenerate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleGenerate")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		regionStr := s.config.DefaultRegion
		if v := r.URL.Query().Get("region"); v != "" {
			regionStr = v
		}
		regions := strings.Split(regionStr, ",")

		if err := s.generate(ctx, regions); err != nil {
			logger.Errorw("generate", "error", err, "regions", regions)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}

func (s *Server) generate(ctx context.Context, regions []string) error {
	for _, r := range regions {
		if err := s.generateKeysInRegion(ctx, r); err != nil {
			return fmt.Errorf("generateKeysInRegion: %w", err)
		}
	}
	return nil
}

func (s *Server) generateKeysInRegion(ctx context.Context, region string) error {
	logger := logging.FromContext(ctx).Named("generateKeysInRegion")

	// We require at least 2 keys because revision only revises a subset of keys,
	// and that subset selects a random sample from (0-len(keys)], and rand panics
	// if you try to generate a random number between 0 and 0 :).
	if s.config.KeysPerExposure < 2 {
		return fmt.Errorf("number of keys to publish must be at least 2")
	}

	// API calls treat region as a list, for legacy regions.
	regions := []string{region}

	traveler := false
	if val, err := util.RandomInt(100); err != nil {
		return fmt.Errorf("failed to determien traveler status: %w", err)
	} else if val < s.config.ChanceOfTraveler {
		traveler = true
	}

	now := time.Now().UTC()
	// Find the valid intervals - starting with today and working backwards
	minInterval := publishmodel.IntervalNumber(timeutils.UTCMidnight(now.Add(-1 * s.config.MaxIntervalAge).Add(24 * time.Hour)))
	curInterval := publishmodel.IntervalNumber(timeutils.UTCMidnight(now))
	intervals := make([]int32, 0, s.config.KeysPerExposure)
	for i := 0; i < s.config.KeysPerExposure && curInterval >= minInterval; i++ {
		intervals = append(intervals, curInterval)
		curInterval -= verifyapi.MaxIntervalCount
	}

	batchTime := now
	for i := 0; i < s.config.NumExposures; i++ {
		logger.Debugf("generating exposure %d of %d", i+1, s.config.NumExposures)

		exposures, err := util.GenerateExposuresForIntervals(intervals)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publish := &verifyapi.Publish{
			Keys:              exposures,
			HealthAuthorityID: "generated.data",
			Traveler:          traveler,
		}

		if s.config.SimulateSameDayRelease {
			sort.Slice(publish.Keys, func(i int, j int) bool {
				return publish.Keys[i].IntervalNumber < publish.Keys[j].IntervalNumber
			})

			lastKey := &publish.Keys[len(publish.Keys)-1]
			newLastDayKey, err := util.RandomExposureKey(lastKey.IntervalNumber, 144, lastKey.TransmissionRisk)
			if err != nil {
				return fmt.Errorf("failed to simulate same day key release: %w", err)
			}

			lastKey.IntervalCount = lastKey.IntervalCount / 2
			publish.Keys = append(publish.Keys, newLastDayKey)
		}

		val, err := util.RandomInt(100)
		if err != nil {
			return fmt.Errorf("failed to decide revised key status: %w", err)
		}
		generateRevisedKeys := val < s.config.ChanceOfKeyRevision

		reportType := verifyapi.ReportTypeClinical
		if !generateRevisedKeys {
			if s.config.ForceConfirmed {
				reportType = verifyapi.ReportTypeConfirmed
			} else {
				reportType, err = util.RandomReportType()
				if err != nil {
					return fmt.Errorf("failed to generate report type: %w", err)
				}
			}
		}

		intervalIdx, err := util.RandomInt(len(publish.Keys) - 1)
		if err != nil {
			return fmt.Errorf("failed to generate symptom onset interval: %w", err)
		}

		claims := verification.VerifiedClaims{
			ReportType:           reportType,
			SymptomOnsetInterval: uint32(publish.Keys[intervalIdx].IntervalNumber),
		}

		result, err := s.transformer.TransformPublish(ctx, publish, regions, &claims, batchTime)
		if err != nil {
			return fmt.Errorf("failed to transform generated exposures: %w", err)
		}

		n, err := s.database.InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
			Incoming:     result.Exposures,
			RequireToken: true,
		})
		if err != nil {
			return fmt.Errorf("failed to write exposure record: %w", err)
		}
		logger.Debugw("generated exposures", "num", n)

		if generateRevisedKeys {
			revisedReportType, err := util.RandomRevisedReportType()
			if err != nil {
				return fmt.Errorf("failed to generate revised report type: %w", err)
			}

			claims.ReportType = revisedReportType
			batchTime = batchTime.Add(s.config.KeyRevisionDelay)

			result, err := s.transformer.TransformPublish(ctx, publish, regions, &claims, batchTime)
			if err != nil {
				return fmt.Errorf("failed to transform generated exposures: %w", err)
			}

			// Build the revision token
			var token pb.RevisionTokenData
			for _, e := range result.Exposures {
				token.RevisableKeys = append(token.RevisableKeys, &pb.RevisableKey{
					TemporaryExposureKey: e.ExposureKey,
					IntervalNumber:       e.IntervalNumber,
					IntervalCount:        e.IntervalCount,
				})
			}

			// Bypass revision token enforcement on generated data.
			n, err := s.database.InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
				Incoming:     result.Exposures,
				Token:        &token,
				RequireToken: true,
			})
			if err != nil {
				return fmt.Errorf("failed to revise exposure record: %w", err)
			}
			logger.Debugw("revised exposures", "num", n)
		}
	}

	return nil
}
