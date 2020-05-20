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
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	exportapi "github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/monolith"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/util"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/testing/enclient"
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

func publishKeys(t *testing.T, request v1alpha1.Publish) {
	requestUrl := "http://localhost:8080/publish"

	resp, err := enclient.PostRequest(requestUrl, request)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Publish request is sent to %v", requestUrl)
}

func exportBatches(t *testing.T) {
	var bts []byte
	requestUrl := "http://localhost:8080/export/create-batches"
	resp, err := enclient.PostRequest(requestUrl, bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Create batches request is sent to %v", requestUrl)
}

func startExportWorkers(t *testing.T) {
	var bts []byte
	requestUrl := "http://localhost:8080/export/do-work"
	resp, err := enclient.PostRequest(requestUrl, bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Export worker request is sent to %v", requestUrl)
}

func publishRequest(keys []v1alpha1.ExposureKey, regions []string) v1alpha1.Publish {
	padding, _ := util.RandomBytes(1000)
	return v1alpha1.Publish{
		Keys:                keys,
		Regions:             regions,
		AppPackageName:      appPackageName,
		VerificationPayload: "Test Authority",
		Padding:             util.ToBase64(padding),
	}
}

func startServer() *monolith.MonoConfig {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	config, err := monolith.RunServer(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	return config
}
