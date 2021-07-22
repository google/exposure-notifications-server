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

package mirror

import (
	"sort"

	"github.com/google/exposure-notifications-server/internal/mirror/model"
)

type FileStatus struct {
	Order         int
	MirrorFile    *model.MirrorFile
	DownloadPath  string
	Filename      string
	LocalFilename string
	Failed        bool
	Saved         bool
}

func (f *FileStatus) needsDelete() bool {
	return f.DownloadPath == ""
}

func (f *FileStatus) needsDownload() bool {
	return f.MirrorFile == nil
}

func sortFileStatus(fs []*FileStatus) {
	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Order < fs[j].Order
	})
}
