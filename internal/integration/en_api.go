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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	exportapi "github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

const appPackageName = "com.example.android.test"

func collectExportResults(t *testing.T, exportDir string) *export.TemporaryExposureKeyExport {
	exportFile := getExportFile(exportDir, t)

	t.Logf("Reading keys data from: %v", exportFile)

	blob, err := ioutil.ReadFile(exportFile)
	if err != nil {
		t.Fatalf("can't read export file: %v", err)
	}

	keyExport, err := exportapi.UnmarshalExportFile(blob)
	if err != nil {
		t.Fatalf("can't extract export data: %v", err)
	}

	return keyExport
}

type EnServerClient struct {
	client *http.Client
}

// Posts requests to the specified url.
// This methods attempts to serialize data argument as a json.
func (server EnServerClient) postRequest(url string, data interface{}) (*http.Response, error) {
	request := bytes.NewBuffer(JSONRequest(data))
	r, err := http.NewRequest("POST", url, request)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := server.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return error upstream.
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to copy error body (%d): %w", resp.StatusCode, err)
		}
		return resp, fmt.Errorf("post request failed with status: %v\n%v", resp.StatusCode, body)
	}

	return resp, nil
}

// Serializes the given argument to json.
func JSONRequest(data interface{}) []byte {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("unable to marshal json payload")
	}
	return jsonData
}

func getExportFile(exportDir string, t *testing.T) string {
	files, err := ioutil.ReadDir(exportDir)
	if err != nil {
		t.Fatalf("Can't read export directory: %v", err)
	}

	archiveFiles := make([]os.FileInfo, 0)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "zip") {
			archiveFiles = append(archiveFiles, f)
		}
	}

	if len(archiveFiles) < 1 {
		t.Fatalf("can't find export archives in %v", exportDir)
	}

	sort.SliceStable(archiveFiles, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
	exportFile := archiveFiles[0]
	return filepath.Join(exportDir, exportFile.Name())
}

func (enServer EnServerClient) PublishKeys(t *testing.T, request verifyapi.Publish) {
	resp, err := enServer.postRequest("/publish", request)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Publish request is sent to %v", "/publish")
}

func (enServer EnServerClient) ExportBatches(t *testing.T) {
	var bts []byte
	requestUrl := "/export/create-batches"
	resp, err := enServer.postRequest(requestUrl, bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Create batches request is sent to %v", requestUrl)
}

func (enServer EnServerClient) StartExportWorkers(t *testing.T) {
	var bts []byte
	requestUrl := "/export/do-work"
	resp, err := enServer.postRequest(requestUrl, bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Export worker request is sent to %v", requestUrl)
}

func publishRequest(keys []verifyapi.ExposureKey, regions []string) verifyapi.Publish {
	padding, _ := util.RandomBytes(1000)
	return verifyapi.Publish{
		Keys:                keys,
		Regions:             regions,
		AppPackageName:      appPackageName,
		VerificationPayload: "Test Authority",
		Padding:             util.ToBase64(padding),
	}
}
