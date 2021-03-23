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
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/hashicorp/go-multierror"

	"go.opencensus.io/stats"
)

const lockPrefix = "export-importer-worker-lock-"

func (s *Server) handleImport() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleImport")

		logger.Debugw("starting export-importer worker")
		defer logger.Debugw("finished export-importer worker")

		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(s.config.MaxRuntime))
		defer cancel()

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("failed to read active configs", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		var merr *multierror.Error

	OUTER:
		for _, cfg := range configs {
			// Check how we're doing on max runtime.
			if deadlinePassed(ctx) {
				logger.Warnw("deadline passed, but there is still work to do")
				break OUTER
			}

			if err := s.runImport(ctx, cfg); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to import %d: %w", cfg.ID, err))
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

		stats.Record(ctx, mImportSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}

func (s *Server) runImport(ctx context.Context, cfg *model.ExportImport) error {
	ctx = metricsWithExportImportID(ctx, cfg.ID)

	lockID := fmt.Sprintf("%s%d", lockPrefix, cfg.ID)

	logger := logging.FromContext(ctx).Named("runImport").
		With("lock", lockID).
		With("config", cfg)

	unlock, err := s.db.Lock(ctx, lockID, s.config.MaxRuntime)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
			logger.Debugw("skipping (already locked)")
			return nil
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logger.Errorw("failed to unlock", "error", err)
		}
	}()

	// Get the list of files that needs to be processed.
	openFiles, err := s.exportImportDB.GetOpenImportFiles(ctx, s.config.ImportLockTime, s.config.ImportRetryRate, cfg)
	if err != nil {
		return fmt.Errorf("failed to read open import files: %w", err)
	}
	if len(openFiles) == 0 {
		return nil
	}

	// Read in public keys.
	keys, err := s.exportImportDB.AllowedKeys(ctx, cfg)
	if err != nil {
		return fmt.Errorf("unable to read public keys: %w", err)
	}
	logger.Debugw("allowed public keys for file", "public_keys", keys)

	var merr *multierror.Error

	var completedFiles, failedFiles int64
OUTER:
	for _, file := range openFiles {
		// Check how we're doing on max runtime.
		if deadlinePassed(ctx) {
			logger.Warnw("deadline passed, but there is still work to do")
			break OUTER
		}

		if err := s.exportImportDB.LeaseImportFile(ctx, s.config.ImportLockTime, file); err != nil {
			logger.Warnw("unexpected race condition, file already locked", "file", file, "error", err)
			return nil
		}

		// import the file.
		status := model.ImportFileComplete
		result, err := s.ImportExportFile(ctx, &ImportRequest{
			config:       s.config,
			exportImport: cfg,
			keys:         keys,
			file:         file,
		})
		if err != nil {
			merr = multierror.Append(merr, err)

			str := fmt.Sprintf("import file error [retry %d]", file.Retries)
			file.Retries++
			if errors.Is(err, ErrArchiveNotFound) {
				str += ", file not found"
			}

			// Check the retries.
			logger.Errorw(str, "exportImportID", cfg.ID, "filename", file.ZipFilename)
			failedFiles++
		}

		// the not found error is passed through.
		if result != nil {
			completedFiles++
			logger.Infow("completed file import", "inserted", result.insertedKeys, "revised", result.revisedKeys, "dropped", result.droppedKeys)
		}

		if err := s.exportImportDB.CompleteImportFile(ctx, file, status); err != nil {
			logger.Errorw("failed to mark file completed", "file", file, "error", err)
		}
	}

	stats.Record(ctx, mFilesImported.M(completedFiles))
	stats.Record(ctx, mFilesFailed.M(failedFiles))
	return merr.ErrorOrNil()
}

func deadlinePassed(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}
	if time.Now().After(deadline) {
		return true
	}
	return false
}
