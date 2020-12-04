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

// Package stats is a model abstraction for health authority telemetry.
package stats

import (
	"log"
	"time"
)

const (
	MaxOldestTEK = 14
	MaxOnsetDays = 28
)

// HealthAuthorityRelease defines a distributed lock, preventing
// stats from being retrieved too frequently.
type HealthAuthorityRelease struct {
	HealthAuthorityID int64
	NotBefore         time.Time
}

// CanPull returns true if the NotBefore time is in the past.
func (h *HealthAuthorityRelease) CanPull() bool {
	return time.Now().UTC().After(h.NotBefore)
}

// HealthAuthorityStats represents the raw metrics for an individual
// health authority for a given hour.
type HealthAuthorityStats struct {
	HealthAuthorityID int64
	Hour              time.Time
	PublishCount      int32
	TEKCount          int32
	RevisionCount     int32
	OldestTekDays     []int32
	OnsetAgeDays      []int32
	MissingOnset      int32
}

// NewHour creates a HealthAuthorityStats record for specified hour.
func InitHour(healthAuthorityID int64, hour time.Time) *HealthAuthorityStats {
	return &HealthAuthorityStats{
		HealthAuthorityID: healthAuthorityID,
		Hour:              hour.UTC().Truncate(time.Hour),
		PublishCount:      0,
		TEKCount:          0,
		RevisionCount:     0,
		OldestTekDays:     make([]int32, MaxOldestTEK+1),
		OnsetAgeDays:      make([]int32, MaxOnsetDays+1),
		MissingOnset:      0,
	}
}

// AddPublish increments the stats for a given hour. This should be called
// inside of a read-modify-write database transaction.
func (has *HealthAuthorityStats) AddPublish(teks int32, revision bool, oldestDays int, onsetDaysAgo int, missingOnset bool) {
	has.PublishCount++
	has.TEKCount += teks
	if revision {
		has.RevisionCount++
	}
	log.Printf("%v %v", oldestDays, len(has.OldestTekDays))
	if oldestDays >= 0 && oldestDays < len(has.OldestTekDays) {
		has.OldestTekDays[oldestDays] = has.OldestTekDays[oldestDays] + 1
	}
	if missingOnset {
		has.MissingOnset++
	} else if onsetDaysAgo >= 0 && onsetDaysAgo < len(has.OnsetAgeDays) {
		has.OnsetAgeDays[onsetDaysAgo] = has.OnsetAgeDays[onsetDaysAgo] + 1
	}
}
