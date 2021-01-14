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

package model

import (
	"time"
)

const (
	ImportFileOpen     = "OPEN"
	ImportFilePending  = "PENDING"
	ImportFileComplete = "COMPLETE"
	ImportFileFailed   = "FAILED"
)

// ImportFile represents an individual export file that is scheduled for,
// or has been attempted or imported into the system.
type ImportFile struct {
	ID             int64
	ExportImportID int64
	ZipFilename    string
	DiscoveredAt   time.Time
	ProcessedAt    *time.Time
	Status         string
	Retries        uint
}

// ShouldTry performs some introspection on an import file from the DB, and
// returns true if that file should be tried for download.
func (f *ImportFile) ShouldTry(retryRate time.Duration) bool {
	if f.Status == ImportFileOpen {
		now := time.Now().UTC()
		d := f.DiscoveredAt.Add(time.Duration(f.Retries) * retryRate)
		return now.After(d)
	}
	return false
}
