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

package mirror

import (
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-server/internal/mirror/model"
)

func TestFileStatus_NeedsDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		downloadPath string
		exp          bool
	}{
		{
			downloadPath: "",
			exp:          true,
		},
		{
			downloadPath: "banana",
			exp:          false,
		},
		{
			downloadPath: "💰",
			exp:          false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("download_path_%s", tc.downloadPath), func(t *testing.T) {
			t.Parallel()

			fs := &FileStatus{DownloadPath: tc.downloadPath}
			if got, want := fs.needsDelete(), tc.exp; got != want {
				t.Errorf("expected %t to be %t", got, want)
			}
		})
	}
}

func TestFileStatus_NeedsDownload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mirrorFile *model.MirrorFile
		exp        bool
	}{
		{
			mirrorFile: nil,
			exp:        true,
		},
		{
			mirrorFile: &model.MirrorFile{},
			exp:        false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("mirror_file_%t", tc.mirrorFile != nil), func(t *testing.T) {
			t.Parallel()

			fs := &FileStatus{MirrorFile: tc.mirrorFile}
			if got, want := fs.needsDownload(), tc.exp; got != want {
				t.Errorf("expected %t to be %t", got, want)
			}
		})
	}
}
