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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

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
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		anyErrors := false
		for _, config := range configs {
			logger.Infow("checking index file", "config", config)

			resp, err := client.Get(config.IndexFile)
			if err != nil {
				anyErrors = true
				logger.Errorf("error downloading index file", "file", config.IndexFile, "error", err)
				continue
			}

			defer resp.Body.Close()
			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				anyErrors = true
				logger.Errorf("unable to read index file", "file", config.IndexFile, "error", err)
				continue
			}
			zipNames := strings.Split(string(bytes), "\n")
			currentFiles := make([]string, 0, len(zipNames))
			for _, zipFile := range zipNames {
				fullZipFile := fmt.Sprintf("%s%s", config.ExportRoot, strings.TrimSpace(zipFile))
				logger.Debugw("found new export file", "zipFile", fullZipFile)
				currentFiles = append(currentFiles, fullZipFile)
			}

			n, err := s.exportImportDB.CreateFiles(ctx, config, currentFiles)
			if err != nil {
				anyErrors = true
				logger.Errorf("unable to write new files", "error", err)
			}
			if n != 0 {
				logger.Infof("found new files for export import config", "file", config.IndexFile, "newCount", n)
			}
		}

		status := http.StatusOK
		if anyErrors {
			status = http.StatusInternalServerError
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(http.StatusText(status)))
	}
}
