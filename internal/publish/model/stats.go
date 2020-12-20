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

// Package model is a model abstraction for health authority telemetry.
package model

import (
	"time"
)

const (
	MaxOldestTEK = 14
	MaxOnsetDays = 28

	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformUnknown = "unknown"
)

var (
	platforms = map[string]int{
		PlatformAndroid: 0,
		PlatformIOS:     1,
		PlatformUnknown: 2,
	}
)

// HealthAuthorityStats represents the raw metrics for an individual
// health authority for a given hour.
type HealthAuthorityStats struct {
	HealthAuthorityID int64
	Hour              time.Time
	PublishCount      []int32
	TEKCount          int32
	RevisionCount     int32
	OldestTekDays     []int32
	OnsetAgeDays      []int32
	MissingOnset      int32
}

// InitHour creates a HealthAuthorityStats record for specified hour.
func InitHour(healthAuthorityID int64, hour time.Time) *HealthAuthorityStats {
	return &HealthAuthorityStats{
		HealthAuthorityID: healthAuthorityID,
		Hour:              hour.UTC().Truncate(time.Hour),
		PublishCount:      make([]int32, len(platforms)),
		TEKCount:          0,
		RevisionCount:     0,
		OldestTekDays:     make([]int32, MaxOldestTEK+1),
		OnsetAgeDays:      make([]int32, MaxOnsetDays+1),
		MissingOnset:      0,
	}
}

// PublishInfo is the paremeters to the AddPublish call
type PublishInfo struct {
	CreatedAt    time.Time
	Platform     string
	NumTEKs      int32
	Revision     bool
	OldestDays   int
	OnsetDaysAgo int
	MissingOnset bool
}

// AddPublish increments the stats for a given hour. This should be called
// inside of a read-modify-write database transaction.
func (has *HealthAuthorityStats) AddPublish(info *PublishInfo) {
	switch info.Platform {
	case PlatformAndroid:
		has.PublishCount[platforms[PlatformAndroid]]++
	case PlatformIOS:
		has.PublishCount[platforms[PlatformIOS]]++
	default:
		has.PublishCount[platforms[PlatformUnknown]]++
	}

	has.TEKCount += info.NumTEKs
	if info.Revision {
		has.RevisionCount++
		return
	}
	// This info is only updated if it's not a revision.
	if info.OldestDays >= 0 && info.OldestDays < len(has.OldestTekDays) {
		has.OldestTekDays[info.OldestDays]++
	}
	if info.MissingOnset {
		has.MissingOnset++
	} else if info.OnsetDaysAgo >= 0 && info.OnsetDaysAgo < len(has.OnsetAgeDays) {
		has.OnsetAgeDays[info.OnsetDaysAgo]++
	}
}
