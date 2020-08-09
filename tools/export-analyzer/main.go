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

// This tool displays the content of the export file.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"time"

	"github.com/google/exposure-notifications-server/internal/export"
	exportpb "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/publish/model"
)

var (
	filePath = flag.String("file", "", "Path to the export zip file.")
	quiet    = flag.Bool("q", false, "run in quiet mode")
)

func main() {
	flag.Parse()

	if *filePath == "" {
		log.Fatal("--file is required.")
	}

	blob, err := ioutil.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("can't read export file: %v", err)
	}

	keyExport, err := export.UnmarshalExportFile(blob)
	if err != nil {
		log.Fatalf("error unmarshaling export file: %v", err)
	}

	// Do some basic data validation.
	if !*quiet {
		checkExportFile(keyExport)
	}

	prettyJSON, err := json.MarshalIndent(keyExport, "", "  ")
	if err != nil {
		log.Fatalf("error pretty printing export: %v", err)
	}
	log.Printf("%v", string(prettyJSON))
}

func checkExportFile(export *exportpb.TemporaryExposureKeyExport) {
	now := time.Now().UTC()
	floor := model.IntervalNumber(now.Add(-14 * 24 * time.Hour))
	ceiling := model.IntervalNumber(now)

	checkKeys("keys", export.Keys, floor, ceiling)
	checkKeys("revisedKeys", export.RevisedKeys, floor, ceiling)
}

func checkKeys(kType string, keys []*exportpb.TemporaryExposureKey, floor, ceiling int32) {
	for i, k := range keys {
		if l := len(k.KeyData); l != 16 {
			log.Printf("%s #%d: invald key length: want 16, got: %v", kType, i, l)
		}
		if s := k.GetRollingStartIntervalNumber(); s < floor {
			log.Printf("%s #%d: rolling interval start number is > 14 days ago, want >= %d, got %d", kType, i, floor, s)
		} else if s > ceiling {
			log.Printf("%s #%d: rolling interval start number in the future, want < %d, got %d", kType, i, ceiling, s)
		}
		if r := k.GetRollingPeriod(); r < 1 || r > 144 {
			log.Printf("%s #%d: rolling period invalid, want >= 1 && <= 144, got %d", kType, i, r)
		}
		if k.DaysSinceOnsetOfSymptoms != nil {
			if d := k.GetDaysSinceOnsetOfSymptoms(); d < -14 || d > 14 {
				log.Printf("%s #%d: days_since_onset_of_symptoms is outside of expectd range, -14..14, got: %d", kType, i, d)
			}
		}
	}
}
