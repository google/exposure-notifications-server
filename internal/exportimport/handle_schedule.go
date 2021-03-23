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

	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

const schedulerLockID = "import-scheduler-lock"

func (s *Server) handleSchedule() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleSchedule").
			With("lock", schedulerLockID)

		logger.Debugw("starting exportimport scheduler")
		defer logger.Debugw("finished exportimport scheduler")

		unlock, err := s.db.Lock(ctx, schedulerLockID, s.config.MaxRuntime)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				logger.Debugw("skipping (already locked)")
				s.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
				return
			}
			logger.Errorw("failed to obtain lock", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}()

		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(s.config.MaxRuntime))
		defer cancel()

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("failed to read active configs", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		var merr *multierror.Error

		for _, cfg := range configs {
			if err := s.syncOne(ctx, cfg); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to sync exportimport config %d: %w", cfg.ID, err))
				continue
			}
		}

		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to sync", "errors", errs)
				s.h.RenderJSON(w, http.StatusInternalServerError, errs)
				return
			}
		}

		stats.Record(ctx, mScheduleSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}

func (s *Server) syncOne(ctx context.Context, cfg *model.ExportImport) error {
	ctx = metricsWithExportImportID(ctx, cfg.ID)

	logger := logging.FromContext(ctx).Named("syncOne").
		With("config", cfg)

	logger.Debugw("syncing index file")
	defer logger.Debugw("finished syncing index file")

	client := &http.Client{
		Timeout: s.config.IndexFileDownloadTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.IndexFile, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to download index file: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download index file: %w", err)
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	numNew, numFailed, err := syncFilesFromIndex(ctx, s.exportImportDB, cfg, string(bytes))
	if err != nil {
		return fmt.Errorf("failed to sync index file contents: %w", err)
	}

	logger.Debugw("sync result", "new_files", numNew, "failed_files", numFailed)
	stats.Record(ctx, mFilesScheduled.M(int64(numNew)))
	return nil
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
