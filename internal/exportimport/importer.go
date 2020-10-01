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
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

const lockPrefix = "import-lock-"

func (s *Server) handleImport(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("exportimport.HandleImport")

	return func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		ctx, cancelFn := context.WithDeadline(r.Context(), time.Now().Add(s.config.MaxRuntime))
		defer cancelFn()
		logger.Info("starting export importer")
		defer func() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}()

		configs, err := s.exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			logger.Errorw("unable to read active configs", "error", err)
		}

		for _, config := range configs {
			// Check how we're doing on max runtime.
			if deadlinePassed(ctx) {
				logger.Errorf("deadline passed, still work to do")
				return
			}

			// Obtain a lock to work on this import config.
			unlock, err := s.db.Lock(ctx, fmt.Sprintf("%s%d", lockPrefix, config.ID), s.config.MaxRuntime)
			if err != nil {
				if errors.Is(err, database.ErrAlreadyLocked) {
					logger.Warnw("import already locked", "config", config)
				}
				logger.Errorw("error locking import config", "config", config, "error", err)
				continue
			}
			defer func() {
				if err := unlock(); err != nil {
					logger.Errorf("failed to unlock: %v", err)
				}
			}()

			// Get the list of files that needs to be processed.
			openFiles, err := s.exportImportDB.GetOpenImportFiles(ctx, s.config.ImportLockTime, config)
			if err != nil {
				logger.Errorf("unable to read open export files", "config", config, "error", err)
			}
			if len(openFiles) == 0 {
				logger.Infow("no work to do", "config", config)
				continue
			}

			// Read in public keys.
			keys, err := s.exportImportDB.AllowedKeys(ctx, config)
			if err != nil {
				logger.Errorf("unable to read public keys", "config", config, "error", err)
			}
			logger.Debugw("allowed public keys for file", "publicKeys", keys)

			for _, file := range openFiles {
				// Check how we're doing on max runtime.
				if deadlinePassed(ctx) {
					logger.Errorf("deadline passed, still work to do", "config", config)
					return
				}

				if err := s.exportImportDB.LeaseImportFile(ctx, s.config.ImportLockTime, file); err != nil {
					logger.Errorf("unexpected race condition, file already locked", "file", file, "error", err)
					continue
				}

				// import the file.
				result, err := s.ImportExportFile(ctx, &ImportRequest{
					config:       s.config,
					exportImport: config,
					keys:         keys,
					file:         file,
				})
				if err != nil {
					logger.Errorf("error processing import file", "error", err)
					continue
				}
				logger.Infow("completed file import", "inserted", result.insertedKeys, "revised", result.revisedKeys, "dropped", result.droppedKeys)

				if err := s.exportImportDB.CompleteImportFile(ctx, file); err != nil {
					logger.Errorf("failed to mark file completed", "file", file, "error", err)
				}
			}
		}

	}
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
