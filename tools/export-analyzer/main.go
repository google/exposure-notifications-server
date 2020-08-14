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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/google/exposure-notifications-server/internal/export"
	exportpb "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/hashicorp/go-multierror"
)

var (
	filePath       = flag.String("file", "", "Path to the export zip file.")
	printJSON      = flag.Bool("json", true, "Print a JSON representation of the output")
	quiet          = flag.Bool("q", false, "run in quiet mode")
	allowedTEKAge  = flag.Duration("tekage", 14*24*time.Hour, "max TEK age in checks")
	symptomDayLmit = flag.Int("symptomdays", 14, "magnitude of expected symptom onset day range")
)

func main() {
	flag.Parse()
	if *filePath == "" {
		log.Fatal("--file is required.")
	}
	if *allowedTEKAge < time.Duration(0) {
		log.Fatalf("--tekage must be a positive duration, got: %v", *allowedTEKAge)
	}
	if *symptomDayLmit < 0 {
		log.Fatalf("--sypmtomdays must be >=0, got: %v", *symptomDayLmit)
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
	success := true
	if err := checkExportFile(keyExport); err != nil {
		success = false
		if !*quiet {
			log.Printf("export file contains errors: %v", err)
		}
	}

	if *printJSON {
		prettyJSON, err := json.MarshalIndent(keyExport, "", "  ")
		if err != nil {
			log.Fatalf("error pretty printing export: %v", err)
		}
		log.Printf("%v", string(prettyJSON))
	}

	if !success {
		// return a non zero code if there are issues with the export file.
		os.Exit(1)
	}
}

func checkExportFile(export *exportpb.TemporaryExposureKeyExport) error {
	now := time.Now().UTC()
	floor := model.IntervalNumber(now.Add(*allowedTEKAge))
	ceiling := model.IntervalNumber(now)

	var errors *multierror.Error
	if err := checkKeys("keys", export.Keys, floor, ceiling); err != nil {
		errors = multierror.Append(errors, err)
	}
	if err := checkKeys("revisedKeys", export.RevisedKeys, floor, ceiling); err != nil {
		errors = multierror.Append(errors, err)
	}
	return errors
}

func checkKeys(kType string, keys []*exportpb.TemporaryExposureKey, floor, ceiling int32) error {
	symptomDays := int32(*symptomDayLmit)
	var errors *multierror.Error
	for i, k := range keys {
		if l := len(k.KeyData); l != 16 {
			errors = multierror.Append(fmt.Errorf("%s #%d: invald key length: want 16, got: %v", kType, i, l))
		}
		if s := k.GetRollingStartIntervalNumber(); s < floor {
			errors = multierror.Append(fmt.Errorf("%s #%d: rolling interval start number is > %v ago, want >= %d, got %d", kType, i, *allowedTEKAge, floor, s))
		} else if s > ceiling {
			errors = multierror.Append(fmt.Errorf("%s #%d: rolling interval start number in the future, want < %d, got %d", kType, i, ceiling, s))
		}
		if r := k.GetRollingPeriod(); r < 1 || r > 144 {
			errors = multierror.Append(fmt.Errorf("%s #%d: rolling period invalid, want >= 1 && <= 144, got %d", kType, i, r))
		}
		if k.DaysSinceOnsetOfSymptoms != nil {
			if d := k.GetDaysSinceOnsetOfSymptoms(); d < -symptomDays || d > symptomDays {
				errors = multierror.Append(fmt.Errorf("%s #%d: days_since_onset_of_symptoms is outside of expectd range, -%d..%d, got: %d", kType, i, symptomDays, symptomDays, d))
			}
		}
	}
	return errors
}
