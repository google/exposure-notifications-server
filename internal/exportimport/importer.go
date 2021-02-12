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

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

const lockPrefix = "import-lock-"

func (s *Server) handleImport() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleImport")

		ctx, span := trace.StartSpan(ctx, "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		ctx, cancelFn := context.WithDeadline(ctx, time.Now().Add(s.config.MaxRuntime))
		defer cancelFn()
		logger.Info("starting export importer")
		defer func() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
		}()
		ctx = logging.WithLogger(ctx, logger)

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("unable to read active configs", "error", err)
		}

		for _, config := range configs {
			// Check how we're doing on max runtime.
			if deadlinePassed(ctx) {
				logger.Warnf("deadline passed, still work to do")
				return
			}

			if err := s.runImport(ctx, config); err != nil {
				logger.Errorw("error running export-import", "config", config, "error", err)
			}
		}

		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) runImport(ctx context.Context, config *model.ExportImport) error {
	logger := logging.FromContext(ctx)

	// Obtain a lock to work on this import config.
	unlock, err := s.db.Lock(ctx, fmt.Sprintf("%s%d", lockPrefix, config.ID), s.config.MaxRuntime)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
			logger.Warnw("import already locked", "config", config)
		}
		logger.Errorw("error locking import config", "config", config, "error", err)
		return nil
	}
	defer func() {
		if err := unlock(); err != nil {
			logger.Errorf("failed to unlock: %v", err)
		}
	}()

	// Get the list of files that needs to be processed.
	openFiles, err := s.exportImportDB.GetOpenImportFiles(ctx, s.config.ImportLockTime, s.config.ImportRetryRate, config)
	if err != nil {
		logger.Errorw("unable to read open import files", "config", config, "error", err)
	}
	if len(openFiles) == 0 {
		logger.Infow("no work to do", "config", config)
		return nil
	}

	// Read in public keys.
	keys, err := s.exportImportDB.AllowedKeys(ctx, config)
	if err != nil {
		return fmt.Errorf("unable to read public keys: %w", err)
	}
	logger.Debugw("allowed public keys for file", "publicKeys", keys)

	errs := []error{}
	for _, file := range openFiles {
		// Check how we're doing on max runtime.
		if deadlinePassed(ctx) {
			return fmt.Errorf("deadline exceeded, work not done")
		}

		if err := s.exportImportDB.LeaseImportFile(ctx, s.config.ImportLockTime, file); err != nil {
			logger.Warnw("unexpected race condition, file already locked", "file", file, "error", err)
			return nil
		}

		// import the file.
		status := model.ImportFileComplete
		result, err := s.ImportExportFile(ctx, &ImportRequest{
			config:       s.config,
			exportImport: config,
			keys:         keys,
			file:         file,
		})
		if err != nil {
			errs = append(errs, err)
			str := fmt.Sprintf("import file error [retry %d]", file.Retries)
			file.Retries++
			if errors.Is(err, ErrArchiveNotFound) {
				str += ", file not found"
			}

			// Check the retries.
			logger.Errorw(str, "exportImportID", config.ID, "filename", file.ZipFilename)
		}
		// the not found error is passed through.
		if result != nil {
			logger.Infow("completed file import", "inserted", result.insertedKeys, "revised", result.revisedKeys, "dropped", result.droppedKeys)
		}

		if err := s.exportImportDB.CompleteImportFile(ctx, file, status); err != nil {
			logger.Errorf("failed to mark file completed", "file", file, "error", err)
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("%d errors processing import file: %v", len(errs), errs)
	}
	return nil
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
