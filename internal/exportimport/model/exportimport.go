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

// Package model is a model abstraction of export import configuration and status.
package model

import (
	"fmt"
	"time"
)

// ExportImport reqpresnts the configuration of a set of export files
// to be imported into this server, by pointing at the index file
// and remote root directory.
type ExportImport struct {
	ID         int64
	IndexFile  string
	ExportRoot string
	Region     string
	From       time.Time
	Thru       *time.Time
}

// Validate checks the contents of an ExportImport file. This is a utility
// function for the admin console.
func (ei *ExportImport) Validate() error {
	if l := len(ei.Region); l == 0 {
		return fmt.Errorf("region cannot be blank")
	} else if l > 5 {
		return fmt.Errorf("region cannot be longer than 5 characters")
	}

	if ei.IndexFile == "" {
		return fmt.Errorf("IndexFile cannot be blank")
	}
	if ei.ExportRoot == "" {
		return fmt.Errorf("ExportRoot cannot be blank")
	}

	return nil
}

// Active returns if the ExportImport configuration is currently
// active based on From and Thru times.
func (ei *ExportImport) Active() bool {
	now := time.Now().UTC()
	return ei.From.Before(now) && (ei.Thru == nil || now.Before(*ei.Thru))
}
