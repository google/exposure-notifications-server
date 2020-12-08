// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mirrors is part of the admin system.
package mirrors

import (
	mirrormodel "github.com/google/exposure-notifications-server/internal/mirror/model"
)

type formData struct {
	Action string `form:"action" binding:"required"`

	IndexFile          string `form:"index-file" binding:"required"`
	ExportRoot         string `form:"export-root"`
	CloudStorageBucket string `form:"cloud-storage-bucket" binding:"required"`
	FilenameRoot       string `form:"filename-root"`
	FilenameRewrite    string `form:"filename-rewrite"`
}

func (f *formData) PopulateMirror(m *mirrormodel.Mirror) error {
	m.IndexFile = f.IndexFile
	m.ExportRoot = f.ExportRoot
	m.CloudStorageBucket = f.CloudStorageBucket
	m.FilenameRoot = f.FilenameRoot
	if f.FilenameRewrite != "" {
		m.FilenameRewrite = &f.FilenameRewrite
	} else {
		m.FilenameRewrite = nil
	}
	return nil
}
