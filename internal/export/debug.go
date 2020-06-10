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

package export

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/logging"
)

func (s *Server) handleDebug(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx)

	type response struct {
		Config           *Config
		ExportConfigs    []*exportmodel.ExportConfig
		ExportBatchEnds  map[int64]time.Time
		ExportBatchFiles []string

		SignatureInfos []*exportmodel.SignatureInfo
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		db := s.env.Database()

		exportConfigs, err := exportdatabase.New(db).GetAllExportConfigs(ctx)
		if err != nil {
			logger.Errorf("failed to get all export configs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		exportBatchEnds := make(map[int64]time.Time, len(exportConfigs))
		for _, ec := range exportConfigs {
			end, err := exportdatabase.New(db).LatestExportBatchEnd(ctx, ec)
			if err != nil {
				logger.Errorf("failed to get latest export batch end for %d: %v", ec.ConfigID, err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
				return
			}
			exportBatchEnds[ec.ConfigID] = end
		}

		exportBatchFiles, err := exportdatabase.New(db).LookupExportFiles(ctx, 4*time.Hour)
		if err != nil {
			logger.Errorf("failed to get export files: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		signatureInfos, err := exportdatabase.New(db).ListAllSigntureInfos(ctx)
		if err != nil {
			logger.Errorf("failed to get all signature infos: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		resp := &response{
			Config:           s.config,
			ExportConfigs:    exportConfigs,
			ExportBatchEnds:  exportBatchEnds,
			ExportBatchFiles: exportBatchFiles,
			SignatureInfos:   signatureInfos,
		}

		w.Header().Set("Content-Type", "application/json")

		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		if err := e.Encode(resp); err != nil {
			panic(err)
		}
	}
}
