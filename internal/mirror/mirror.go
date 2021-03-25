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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	mirrordatabase "github.com/google/exposure-notifications-server/internal/mirror/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

const mirrorLockPrefix = "mirror-lock"

type Response struct {
	Mirrors []*Status `json:"mirrors"`
}

type Status struct {
	ID        int64    `json:"id"`
	Processed bool     `json:"processed"`
	Errors    []string `json:"errors,omitempty"`
}

func (s *Server) handleMirror() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleMirror")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ctx, cancel := context.WithTimeout(ctx, s.config.MaxRuntime)
		defer cancel()

		// Chop off 30 seconds to save state at the end.
		runtime := s.config.MaxRuntime
		if runtime > time.Minute {
			runtime = runtime - (30 * time.Second)
		}
		deadline := time.Now().Add(runtime)
		logger.Infow("mirror will run until", "deadline", deadline)

		// Get all possible mirror candidates.
		mirrors, err := s.mirrorDB.Mirrors(ctx)
		if err != nil {
			logger.Errorw("failed to list mirrors", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		// Start building a response for all mirrors.
		var resp Response
		var hasError bool

		for _, mirror := range mirrors {
			// Build the initial status
			status := &Status{ID: mirror.ID}

			// If the deadline has passed, do not attempt to process additional
			// mirrors. Note that exceeding the deadline is NOT considered an error.
			if deadlinePassed(deadline) {
				status.Errors = []string{"deadline exceeded before processing"}
				resp.Mirrors = append(resp.Mirrors, status)
				continue
			}

			// Process the mirror.
			if err := s.processMirror(ctx, deadline, mirror); err != nil {
				hasError = true

				status.Processed = true
				status.Errors = []string{err.Error()}

				// If the response was a multierror, extract into the list.
				var merr *multierror.Error
				if errors.As(err, &merr) {
					errMsgs := make([]string, len(merr.Errors))
					for i, v := range merr.Errors {
						errMsgs[i] = v.Error()
					}
					status.Errors = errMsgs
				}

				resp.Mirrors = append(resp.Mirrors, status)
				continue
			}

			// If we got this far, the job completed successfully.
			status.Processed = true
		}

		if hasError {
			logger.Errorw("failed to process al mirrors", "response", resp)
			s.h.RenderJSON(w, http.StatusInternalServerError, resp)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, resp)
	})
}

// processMirror processes all files in the mirror, deleting files that have
// been removed and downloading the new index and exports.
//
// An ambitious engineer might say "wow, we should really parallelize the
// download and upload steps". Please don't. The files need to be processed and
// uploaded in the order, in case a device is in the middle of downloading
// things. Some devices uses the timestamps and ordering of the files, and
// uploading in parallel wouldn't guarantee upload order. Thus, a client could
// skip files and that would be bad.
func (s *Server) processMirror(ctx context.Context, deadline time.Time, mirror *model.Mirror) (retErr error) {
	logger := logging.FromContext(ctx).Named("processMirror").
		With("mirror_id", mirror.ID)
	blobstore := s.env.Blobstore()

	lockID := fmt.Sprintf("%s-%d", mirrorLockPrefix, mirror.ID)
	unlock, err := s.db.Lock(ctx, lockID, s.config.MaxRuntime)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
			logger.Infow("mirror already locked, skipping")
			return nil
		}
		return fmt.Errorf("failed to lock mirror: %w", err)
	}
	defer func() {
		logger.Debugw("finished processing mirror", "index", mirror.IndexFile)

		if err := unlock(); err != nil {
			retErr = fmt.Errorf("failed to unlock mirror: %w, original error: %s", err, retErr)
		}
	}()

	// If we got this far, we have the lock, start processing.
	logger.Infow("starting mirror", "index", mirror.IndexFile)

	// Get the list of known mirror files.
	knownFiles, err := s.mirrorDB.ListFiles(ctx, mirror.ID)
	if err != nil {
		return fmt.Errorf("failed to list mirror files: %w", err)
	}

	// Download the index, which will return the fully qualified download links
	// for the files.
	indexFiles, err := s.downloadIndex(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to download index: %w", err)
	}

	// Build a running list of all errors. In many cases, processing continues on
	// error.
	var merr *multierror.Error

	// Figure out actions to take by diffing the known files and current files.
	actions := computeActions(knownFiles, indexFiles)

	// Delete any stale export files. These are files that we previously mirrored,
	// but are no longer in the index.
	for filename, status := range actions {
		if status.needsDelete() {
			// Use a function here so defers run sooner.
			func() {
				localFilename := filename
				if status.MirrorFile != nil && status.MirrorFile.LocalFilename != nil {
					val := *status.MirrorFile.LocalFilename
					if val != "" {
						localFilename = val
					}
				}

				logger.Debugw("deleting stale export file",
					"upstream_file", filename,
					"local_file", localFilename,
					"bucket", mirror.CloudStorageBucket)

				ctx, cancel := context.WithTimeout(ctx, s.config.ExportFileDeleteTimeout)
				defer cancel()

				parent := mirror.CloudStorageBucket
				pth := urlJoin(mirror.FilenameRoot, localFilename)
				if err := blobstore.DeleteObject(ctx, parent, pth); err != nil {
					merr = multierror.Append(merr, fmt.Errorf("failed to delete stale export file %s: %w", urlJoin(parent, pth), err))
					return
				}

				// Remove from map, so will be deleted from the database later. Note we
				// only do this on successful deletion. The item is not removed from the
				// map if deletion fails.
				delete(actions, filename)
			}()
		}
	}

	// Start building a list of all objects - we'll use this to save back to the
	// database later.
	indexObjects := make([]*FileStatus, 0, len(actions))

	// Bypass files that we already know about, since they do not need to be
	// downloaded. This ensures that if we timeout, files we previously knew about
	// are still saved.
	for _, status := range actions {
		if !status.needsDownload() {
			status.Saved = true
			indexObjects = append(indexObjects, status)
		}
	}

	// Download any files we don't have.
	logger.Debugw("mirror state",
		"total", len(actions),
		"existing", len(indexObjects),
		"remaining", len(actions)-len(indexObjects))

	// Convert actions to an array, so we can process files in order. This is
	// necessary if files are being renamed and using timestamps, and if clients
	// are depending on monotonically increasing timestamps.
	remainingWork := make([]*FileStatus, 0, len(actions))
	for _, status := range actions {
		if status.needsDownload() {
			remainingWork = append(remainingWork, status)
		}
	}
	sortFileStatus(remainingWork)

	// Process downloads.
	for _, status := range remainingWork {
		if deadlinePassed(deadline) {
			logger.Warnw("mirror did not complete processing before timeout")
			break
		}

		filename := status.Filename
		logger.Debugw("downloading export file",
			"file", filename,
			"download_path", status.DownloadPath)

		b, err := downloadFile(ctx, status.DownloadPath, s.config.ExportFileDownloadTimeout, s.config.MaxZipBytes)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to download export file %s: %w", filename, err))
			status.Failed = true
			continue
		}

		// See if we need to rewrite the filename.
		writeFilename, err := mirror.RewriteFilename(filename)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to rewrite filename %s: %w", filename, err))
			status.Failed = true
			continue
		}
		status.LocalFilename = writeFilename

		// Write this file to the blobstore - use a function so defers run when
		// expected.
		if err := func() error {
			ctx, cancel := context.WithTimeout(ctx, s.config.ExportFileUploadTimeout)
			defer cancel()

			objName := urlJoin(mirror.FilenameRoot, writeFilename)
			return blobstore.CreateObject(ctx, mirror.CloudStorageBucket, objName, b, true, storage.ContentTypeZip)
		}(); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to write %s to blobstore: %w", filename, err))
			status.Failed = true
			continue
		}

		status.Saved = true
		logger.Debugw("successfully saved mirrored archive",
			"upstream_file", filename,
			"local_file", writeFilename)

		indexObjects = append(indexObjects, status)
	}

	// Build and write a new index file.
	sort.Slice(indexObjects, func(i, j int) bool {
		return indexObjects[i].Order < indexObjects[j].Order
	})

	syncFilenames := make([]*mirrordatabase.SyncFile, 0, len(indexObjects))
	filenames := make([]string, 0, len(indexObjects))
	for _, obj := range indexObjects {
		// Only persist state of the files we got to.
		if obj.Saved {
			syncFilenames = append(syncFilenames, &mirrordatabase.SyncFile{
				RemoteFile: obj.Filename,
				LocalFile:  obj.LocalFilename,
			})
			filenames = append(filenames, urlJoin(mirror.FilenameRoot, obj.LocalFilename))
		}
	}

	bsCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	parent := mirror.CloudStorageBucket
	pth := urlJoin(mirror.FilenameRoot, "index.txt")
	data := []byte(strings.Join(filenames, "\n"))
	if err := blobstore.CreateObject(bsCtx, parent, pth, data, true, storage.ContentTypeTextPlain); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to write index.txt to blobstore: %w", err))
	}

	// Save state to database.
	if err := s.mirrorDB.SaveFiles(ctx, mirror.ID, syncFilenames); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to save index state to database: %w", err))
	}

	retErr = merr.ErrorOrNil()
	return
}

// computeActions computes the actions to take based on the provided known files
// and files in the index.
func computeActions(knownFiles []*model.MirrorFile, indexFiles []string) map[string]*FileStatus {
	actions := make(map[string]*FileStatus, len(indexFiles))
	for _, kf := range knownFiles {
		actions[kf.Filename] = &FileStatus{
			MirrorFile: kf,
		}
	}

	for i, indexFile := range indexFiles {
		lastIdx := strings.LastIndex(indexFile, "/")
		fileName := indexFile[lastIdx+1:]

		// Start ordering a 1 because a value of 0 is also the default value.
		order := i + 1

		if cur, ok := actions[fileName]; ok {
			cur.DownloadPath = indexFile
			cur.Filename = fileName
			cur.LocalFilename = fileName
			if cur.MirrorFile.LocalFilename != nil {
				cur.LocalFilename = *cur.MirrorFile.LocalFilename
			}
			cur.Order = order
		} else {
			actions[fileName] = &FileStatus{
				Order:        order,
				Filename:     fileName,
				DownloadPath: indexFile,
			}
		}
	}

	return actions
}

// downloadFile downloads the file from the given URL u up to maxBytes. If the
// URL does not return a 200, an error is returned. If the process takes longer
// than the provided timeout, an error is returned. If more bytes remain after
// maxBytes, an error is returned. Otherwise, the raw bytes are returned.
func downloadFile(ctx context.Context, u string, timeout time.Duration, maxBytes int64) ([]byte, error) {
	client := &http.Client{Timeout: timeout}

	// Start the download.
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request %s: %w", u, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", u, err)
	}
	defer resp.Body.Close()

	// Ensure a 200 response.
	if code := resp.StatusCode; code != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: status %d", u, code)
	}

	// Create the limited reader.
	var b bytes.Buffer
	r := &io.LimitedReader{R: resp.Body, N: maxBytes}
	if _, err := io.Copy(&b, r); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", u, err)
	}
	if r.N == 0 {
		// Check if there's more data to be read and return an error if so.
		if _, err := r.R.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("failed to read %s: response exceeds %d bytes", u, maxBytes)
		}
	}

	// Return the bytes.
	return b.Bytes(), nil
}

// downloadIndex downloads the index files and returns the list of entries of
// each line. An index file has the format:
//
//   us/1605818705-1605819005-00001.zip
//   us/1605818705-1605819005-00002.zip
//   us/1605818705-1605819005-00003.zip
//   ...
//
// The values are returned in the order in which they appear in the file, joined
// with the configured mirror ExportRoot.
func (s *Server) downloadIndex(ctx context.Context, mirror *model.Mirror) ([]string, error) {
	b, err := downloadFile(ctx, mirror.IndexFile, s.config.IndexFileDownloadTimeout, s.config.MaxIndexBytes)
	if err != nil {
		return nil, err
	}

	currentFiles := make([]string, 0, 64)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		fullName := urlJoin(mirror.ExportRoot, scanner.Text())
		currentFiles = append(currentFiles, fullName)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan index file: %w", err)
	}

	return currentFiles, nil
}

// urlJoin joins a root path with the extra path, attempting to clean leading
// and trailing slashes.
func urlJoin(root, extra string) string {
	// We can't use path.Join here because it strips URLs (for example, "http://"
	// becomes "http:/").
	root = strings.TrimRight(root, "/")
	extra = strings.TrimLeft(extra, "/")
	return strings.TrimLeft(root+"/"+extra, "/")
}

func deadlinePassed(deadline time.Time) bool {
	return time.Now().After(deadline)
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
