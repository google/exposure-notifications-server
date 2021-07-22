// Copyright 2020 the Exposure Notifications Server authors
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

// Package model is a model abstraction of mirror data structures.
package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TimestampRW = "[timestamp]"
	UUID        = "[uuid]"

	// Test is used for testing only.
	Test = "[test]"
)

// Mirror represents an individual mirror configuration.
type Mirror struct {
	ID int64
	// Read from
	IndexFile  string
	ExportRoot string
	// Write to
	CloudStorageBucket string
	FilenameRoot       string
	// Rewrite rules
	FilenameRewrite *string

	// internal state for assigning filenames.
	lastTimestamp int64
}

func (m *Mirror) NeedsRewrite() bool {
	return m.FilenameRewrite != nil
}

// Ensures that for quick mirroring that filenames are monotonically increasing.
func (m *Mirror) nextTimestamp() int64 {
	ret := time.Now().Unix()
	for ret == m.lastTimestamp {
		time.Sleep(100 * time.Millisecond)
		ret = time.Now().Unix()
	}
	m.lastTimestamp = ret
	return ret
}

func (m *Mirror) RewriteFilename(fName string) (string, error) {
	if m.FilenameRewrite == nil {
		return fName, nil
	}

	fName = *m.FilenameRewrite
	for strings.Contains(fName, TimestampRW) {
		fName = strings.Replace(fName, TimestampRW, fmt.Sprintf("%d", m.nextTimestamp()), 1)
	}
	for strings.Contains(fName, UUID) {
		newID, err := uuid.NewRandom()
		if err != nil {
			return "", fmt.Errorf("uuid.NewRandom: %w", err)
		}
		fName = strings.Replace(fName, UUID, strings.ToUpper(newID.String()), 1)
	}
	for strings.Contains(fName, Test) {
		fName = strings.Replace(fName, Test, "TEST", 1)
	}

	if fName == *m.FilenameRewrite {
		return "", fmt.Errorf("mirror filename rewrite pattern contains no replacement patterns")
	}

	return fName, nil
}

// MirrorFile represents a file that we believe to be in the remote index.txt and is mirrored to our CDN.
type MirrorFile struct {
	MirrorID      int64
	Filename      string
	LocalFilename *string
}
