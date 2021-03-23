// Copyright 2021 Google LLC
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

package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/kelseyhightower/run"
	"go.opencensus.io/stats"
)

const backupDatabaseLockID = "backup-database-lock" // TODO

func (s *Server) handleBackup() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("backup.HandleBackup")

		req, err := s.buildBackupRequest(ctx)
		if err != nil {
			logger.Errorw("failed to build request", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		unlock, err := s.db.Lock(ctx, backupDatabaseLockID, s.config.MinTTL)
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
		releaseLock := func() {
			// Note that we explicitly don't release the lock because it allows us to
			// enforce the minimum TTL. By setting the maximum TTL to the minimum TTL
			// and not releasing the lock, it means the next request will fail to
			// acquire the lock (because it is held). However, after the TTL elapses,
			// the lock will have been expired and the run can successfully continue.
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}

		if err := s.executeBackup(req); err != nil {
			defer releaseLock()
			logger.Errorw("failed to execute backup", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}

func (s *Server) buildBackupRequest(ctx context.Context) (*http.Request, error) {
	u, err := url.Parse(s.config.DatabaseInstanceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database instance url: %w", err)
	}
	u.Path = path.Join(u.Path, "export")

	token, err := s.authorizationToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization token: %w", err)
	}

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&backupRequest{
		ExportContext: &exportContext{
			FileType:  "SQL",
			URI:       fmt.Sprintf("gs://%s/database/%s", s.config.Bucket, s.config.DatabaseName),
			Databases: []string{s.config.DatabaseName},

			// Specifically disable offloading because we want this request to run
			// in-band so we can verify the return status.
			Offload: false,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to create body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	return req, nil
}

func (s *Server) authorizationToken(ctx context.Context) (string, error) {
	if v := s.overrideAuthToken; v != "" {
		return v, nil
	}

	token, err := run.Token([]string{"https://www.googleapis.com/auth/cloud-platform"})
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	return token.AccessToken, nil
}

// executeBackup calls the backup API. This is a *blocking* operation that can
// take O(minutes) in some cases.
func (s *Server) executeBackup(req *http.Request) error {
	client := &http.Client{
		Timeout: s.config.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unsuccessful response from backup (got %d, want %d): %s", got, want, b)
	}

	return nil
}

type backupRequest struct {
	ExportContext *exportContext `json:"exportContext"`
}

type exportContext struct {
	FileType  string   `json:"fileType"`
	URI       string   `json:"uri"`
	Databases []string `json:"databases"`
	Offload   bool     `json:"offload"`
}
