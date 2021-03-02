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
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

const schedulerLockID = "import-scheduler-lock"

func (s *Server) handleSchedule() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleSchedule").
			With("lock", schedulerLockID)

		ctx, span := trace.StartSpan(ctx, "(*exportimport.handleSchedule).ServeHTTP")
		defer span.End()

		unlock, err := s.db.Lock(ctx, schedulerLockID, s.config.MaxRuntime)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				w.WriteHeader(http.StatusOK) // don't report conflict/failure to scheduler (will retry later)
				return
			}
			logger.Errorw("failed to obtain lock", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}()

		ctx, cancelFn := context.WithDeadline(ctx, time.Now().Add(s.config.MaxRuntime))
		defer cancelFn()
		logger.Info("starting export import scheduler")

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("unable to read active configs", "error", err)
			return
		}

		httpClient := &http.Client{
			Timeout: s.config.IndexFileDownloadTimeout,
		}

		anyErrors := false
		for _, config := range configs {
			logger.Infow("checking index file", "config", config)

			req, err := http.NewRequestWithContext(ctx, "GET", config.IndexFile, nil)
			if err != nil {
				logger.Errorw("failed to create request to download index file", "file", config.IndexFile, "error", err)
				continue
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				anyErrors = true
				logger.Errorw("error downloading index file", "file", config.IndexFile, "error", err)
				continue
			}

			defer resp.Body.Close()
			bytes, err := io.ReadAll(resp.Body)
			if err != nil {
				anyErrors = true
				logger.Errorw("unable to read index file", "file", config.IndexFile, "error", err)
				continue
			}

			if n, f, err := syncFilesFromIndex(ctx, s.exportImportDB, config, string(bytes)); err != nil {
				logger.Errorw("error syncing index file contents", "exportImportID", config.ID, "error", err)
				anyErrors = true
			} else {
				logger.Infow("import index sync result", "exportImportID", config.ID, "index", config.IndexFile, "newFiles", n, "failedFiles", f)
			}
		}

		status := http.StatusOK
		if anyErrors {
			status = http.StatusInternalServerError
		}
		w.WriteHeader(status)
		fmt.Fprint(w, http.StatusText(status))
	})
}

func buildArchiveURLs(ctx context.Context, config *model.ExportImport, index string) ([]string, error) {
	zipNames := strings.Split(index, "\n")
	currentFiles := make([]string, 0, len(zipNames))
	for _, zipFile := range zipNames {
		if len(project.TrimSpaceAndNonPrintable(zipFile)) == 0 {
			// drop blank lines.
			continue
		}

		// Parse the export root to see if there is a defined path element.
		base, err := url.Parse(config.ExportRoot)
		if err != nil {
			return nil, fmt.Errorf("config.ExportRoot is invalid: %s: %w", config.ExportRoot, err)
		}
		base.Path = path.Join(base.Path, "/", project.TrimSpaceAndNonPrintable(zipFile))
		proposedURL := base.String()
		// Re-parse combined URL in case there are issues with the filename in the index file.
		url, err := url.Parse(proposedURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL constructed: %s: %w", proposedURL, err)
		}
		url.Path = path.Clean(url.Path)
		currentFiles = append(currentFiles, url.String())
	}
	return currentFiles, nil
}

func syncFilesFromIndex(ctx context.Context, db *exportimportdb.ExportImportDB, config *model.ExportImport, index string) (int, int, error) {
	currentFiles, err := buildArchiveURLs(ctx, config, index)
	if err != nil {
		return 0, 0, err
	}

	n, f, err := db.CreateNewFilesAndFailOld(ctx, config, currentFiles)
	if err != nil {
		return 0, 0, fmt.Errorf("error syncing export filenames to database: %w", err)
	}
	return n, f, nil
}
