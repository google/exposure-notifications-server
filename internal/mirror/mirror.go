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

package mirror

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

const mirrorLockPrefix = "mirror-lock"

type FileStatus struct {
	Order        int
	MirrorFile   *model.MirrorFile
	DownloadPath string
	Filename     string
	Failed       bool
	Saved        bool
}

func (f *FileStatus) needsDelete() bool {
	return f.DownloadPath == ""
}

func (f *FileStatus) needsDownload() bool {
	return f.MirrorFile == nil
}

func (s *Server) downloadIndex(w http.ResponseWriter, mirror *model.Mirror) ([]string, error) {
	client := &http.Client{
		Timeout: s.config.IndexFileDownloadTimeout,
	}
	resp, err := client.Get(mirror.IndexFile)
	if err != nil {
		return nil, fmt.Errorf("error downloading index file: %w", err)
	}
	defer resp.Body.Close()
	resp.Body = http.MaxBytesReader(w, resp.Body, s.config.MaxIndexBytes)
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading index file: %w", err)
	}
	zipNames := strings.Split(string(bytes), "\n")
	currentFiles := make([]string, 0, len(zipNames))
	for _, zipFile := range zipNames {
		fullZipFile := fmt.Sprintf("%s%s", mirror.ExportRoot, strings.TrimSpace(zipFile))
		currentFiles = append(currentFiles, fullZipFile)
	}

	return currentFiles, nil
}

func (s *Server) handleMirror(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("mirror.handleMirror")

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := logging.WithLogger(r.Context(), logger)
		_, span := trace.StartSpan(ctx, "(*mirror.handleMirror).ServeHTTP")
		defer span.End()

		ctx, cancelFn := context.WithTimeout(ctx, s.config.MaxRuntime)
		defer cancelFn()

		// Chop of 30 seconds to save state at the end.
		runtime := s.config.MaxRuntime
		if runtime > time.Minute {
			runtime = runtime - (30 * time.Second)
		}
		deadline := time.Now().Add(runtime)
		logger.Infow("mirrow will run until", "deadline", deadline)

		// get all possible mirror candidates
		mirrors, err := s.mirrorDB.Mirrors(ctx)
		if err != nil {
			logger.Errorw("unable to list mirrors", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, mirror := range mirrors {
			if deadlinePassed(deadline) {
				logger.Warnw("mirror timed out before processing all configurations")
				break
			}

			if err := s.processMirror(ctx, w, deadline, mirror); err != nil {
				logger.Errorw("unable to process mirror", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		status := http.StatusOK
		w.WriteHeader(status)
		w.Write([]byte(http.StatusText(status)))
	}
}

func (s *Server) processMirror(ctx context.Context, w http.ResponseWriter, deadline time.Time, mirror *model.Mirror) error {
	logger := logging.FromContext(ctx)

	lockID := fmt.Sprintf("%s-%d", mirrorLockPrefix, mirror.ID)
	unlock, err := s.db.Lock(ctx, lockID, s.config.MaxRuntime)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
			logger.Infow("mirror already locked, skipping", "mirrorID", mirror.ID)
			return nil
		}
		return fmt.Errorf("unable to lock mirror: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			logger.Errorw("failed to unlock: %v", err)
		}
	}()

	logger.Info("starting mirror", "mirrorID", mirror.ID, "index", mirror.IndexFile)

	// have the lock
	knownFiles, err := s.mirrorDB.ListFiles(ctx, mirror.ID)
	if err != nil {
		logger.Errorw("unable to list mirror files", "error", err)
		return nil
	}

	// download the index
	// curFiles contains the fullly qualified download link for the files.
	curFiles, err := s.downloadIndex(w, mirror)
	if err != nil {
		logger.Errorw("unable to download index.txt - skipping", "mirrorID", mirror.ID, "error", err)
		return nil
	}

	// figure out the actions
	actions := make(map[string]*FileStatus, len(curFiles))
	for _, kf := range knownFiles {
		actions[kf.Filename] = &FileStatus{
			MirrorFile: kf,
		}
	}
	for i, curFile := range curFiles {
		lastIdx := strings.LastIndex(curFile, "/")
		fileName := curFile[lastIdx+1:]
		if cur, ok := actions[fileName]; ok {
			cur.DownloadPath = curFile
			cur.Filename = fileName
			cur.Order = i
		} else {
			actions[fileName] = &FileStatus{
				Order:        i,
				Filename:     fileName,
				DownloadPath: curFile,
			}
		}
	}

	// save for convenience.
	blobstore := s.env.Blobstore()

	// delete any files we no longer have
	for filename, status := range actions {
		if status.needsDelete() {
			logger.Infow("deleting stale export file", "file", filename)

			bsCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			// If there's a race and the file was already deleted, no error is returned.
			if err := blobstore.DeleteObject(bsCtx, mirror.CloudStorageBucket, filename); err != nil {
				logger.Errorw("unable to delete file", "bucket", mirror.CloudStorageBucket, "file", filename, "error", err)
				return nil
			}
			// Remove from map, so will be deleted from db.
			delete(actions, filename)
		}
	}

	// download any files we don't have
	indexObjects := make([]*FileStatus, 0, len(actions))
	// quickly bypass anything that doesn't need to be downloaded.
	// This ensures that if we timeout, files we previously knew about remain saved.
	for _, status := range actions {
		if !status.needsDownload() {
			status.Saved = true
			indexObjects = append(indexObjects, status)
		}
	}
	logger.Infow("mirror state", "mirrorID", mirror.ID, "total", len(actions), "existing", len(indexObjects), "remaining", len(actions)-len(indexObjects))

	// go through again and process downloads.
	for filename, status := range actions {
		if deadlinePassed(deadline) {
			logger.Warnw("mirror didn't complete processing before timeout", "mirrorID", mirror.ID)
			break
		}

		if status.needsDownload() {
			logger.Infow("mirroring export file", "mirrorID", mirror.ID, "file", filename)

			client := &http.Client{
				Timeout: s.config.ExportFileDownloadTimeout,
			}
			resp, err := client.Get(status.DownloadPath)
			if err != nil {
				logger.Errorw("unable to download file", "filename", filename, "error", err)
				status.Failed = true
				return nil
			}

			defer resp.Body.Close()
			resp.Body = http.MaxBytesReader(w, resp.Body, s.config.MaxZipBytes)
			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Errorw("unable to read file", "filename", filename, "error", err)
				status.Failed = true
				return nil
			}

			// Write this file to the blob store.
			bsCtx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			objName := fmt.Sprintf("%s/%s", mirror.FilenameRoot, status.Filename)
			if err := blobstore.CreateObject(bsCtx, mirror.CloudStorageBucket, objName, bytes, true, storage.ContentTypeZip); err != nil {
				logger.Errorw("unable to write file to blobstore", "filename", filename, "error", err)
				status.Failed = true
				return nil
			}

			status.Saved = true
			indexObjects = append(indexObjects, status)
		}
	}

	// Write new index file
	sort.Slice(indexObjects, func(i, j int) bool {
		return indexObjects[i].Order < indexObjects[j].Order
	})
	shortFilenames := make([]string, 0, len(indexObjects))
	filenames := make([]string, 0, len(indexObjects))
	for _, obj := range indexObjects {
		// Only persist state of the files we got to.
		if obj.Saved {
			shortFilenames = append(shortFilenames, obj.Filename)
			filenames = append(filenames, fmt.Sprintf("%s/%s", mirror.FilenameRoot, obj.Filename))
		}
	}
	data := []byte(strings.Join(filenames, "\n"))
	indexName := fmt.Sprintf("%s/index.txt", mirror.FilenameRoot)
	bsCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := blobstore.CreateObject(bsCtx, mirror.CloudStorageBucket, indexName, data, true, storage.ContentTypeTextPlain); err != nil {
		logger.Errorw("unable to write index to blobstore", "filename", "index.txt", "error", err)
		return nil
	}

	// sync state to DB.
	if err := s.mirrorDB.SaveFiles(ctx, mirror.ID, shortFilenames); err != nil {
		logger.Errorw("error saving index state to database", "mirrorID", mirror.ID, "error", err)
	}
	logger.Infow("finished mirror", "mirrorID", mirror.ID, "index", mirror.IndexFile)
	return nil
}

func deadlinePassed(deadline time.Time) bool {
	return time.Now().After(deadline)
}
