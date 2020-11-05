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

package exportimport

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

const schedulerLockID = "import-scheduler-lock"

func (s *Server) handleSchedule(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("exportimport.HandleSchedule")

	return func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.StartSpan(r.Context(), "(*exportimport.handleSchedule).ServeHTTP")
		defer span.End()

		unlock, err := s.db.Lock(ctx, schedulerLockID, s.config.MaxRuntime)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				w.WriteHeader(http.StatusOK)
				return
			}
			logger.Warn(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorf("failed to unlock: %v", err)
			}
		}()

		ctx, cancelFn := context.WithDeadline(r.Context(), time.Now().Add(s.config.MaxRuntime))
		defer cancelFn()
		logger.Info("starting export import scheduler")

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("unable to read active configs", "error", err)
			return
		}

		client := &http.Client{
			Timeout: s.config.IndexFileDownloadTimeout,
		}

		anyErrors := false
		for _, config := range configs {
			logger.Infow("checking index file", "config", config)

			resp, err := client.Get(config.IndexFile)
			if err != nil {
				anyErrors = true
				logger.Errorw("error downloading index file", "file", config.IndexFile, "error", err)
				continue
			}

			defer resp.Body.Close()
			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				anyErrors = true
				logger.Errorw("unable to read index file", "file", config.IndexFile, "error", err)
				continue
			}

			if n, err := syncFilesFromIndex(ctx, s.exportImportDB, config, string(bytes)); err != nil {
				logger.Errorw("error syncing index file contents", "exportImportID", config.ID, "error", err)
				anyErrors = true
			} else {
				logger.Infow("import index sync result", "exportImportID", config.ID, "index", config.IndexFile, "newFiles", n)
			}
		}

		status := http.StatusOK
		if anyErrors {
			status = http.StatusInternalServerError
		}
		w.WriteHeader(status)
		w.Write([]byte(http.StatusText(status)))
	}
}

func syncFilesFromIndex(ctx context.Context, db *exportimportdb.ExportImportDB, config *model.ExportImport, index string) (int, error) {
	zipNames := strings.Split(index, "\n")
	currentFiles := make([]string, 0, len(zipNames))
	for _, zipFile := range zipNames {
		if len(strings.TrimSpace(zipFile)) == 0 {
			// drop blank lines.
			continue
		}

		// Parse the export root to see if there is a defined path element.
		base, err := url.Parse(config.ExportRoot)
		if err != nil {
			return 0, fmt.Errorf("config.ExportRoot is invalid: %s: %w", config.ExportRoot, err)
		}
		relativePath := path.Join(base.Path, "/", strings.TrimSpace(zipFile))
		// Build and parse the proposed absolute path.
		proposedURL := fmt.Sprintf("%s%s", config.ExportRoot, relativePath)
		url, err := url.Parse(proposedURL)
		if err != nil {
			return 0, fmt.Errorf("invalid URL constructed: %s: %w", proposedURL, err)
		}
		url.Path = path.Clean(url.Path)
		currentFiles = append(currentFiles, url.String())
	}

	n, err := db.CreateFiles(ctx, config, currentFiles)
	if err != nil {
		return 0, fmt.Errorf("error syncing export filenames to database: %w", err)
	}
	return n, nil
}
