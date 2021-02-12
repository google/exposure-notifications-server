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
	"os"
	"path/filepath"
	"time"

	"github.com/google/exposure-notifications-server/internal/export"
	exportpb "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/hashicorp/go-multierror"
)

var (
	showSig         = flag.Bool("sig", true, "show signature information from export bundle in json")
	filePath        = flag.String("file", "", "path to the export files, supports file globs")
	printJSON       = flag.Bool("json", true, "show the export in json")
	quiet           = flag.Bool("q", false, "run in quiet mode")
	allowedTEKAge   = flag.Duration("tek-age", 14*24*time.Hour, "max TEK age in checks")
	symptomDayLimit = flag.Int("symptom-days", 14, "magnitude of expected symptom onset day range")
	fileAge         = flag.Duration("file-age", time.Duration(0), "file age is a positive duration that indicates how old a file is, this would be added to tek-age when validating the file and adjusts 'current time' for validing future keys.")
)

func main() {
	if err := realMain(); err != nil {
		printError("%s", err)
		os.Exit(1)
	}
}

func realMain() error {
	flag.Parse()
	if *filePath == "" {
		return fmt.Errorf("--file is required")
	}
	if *allowedTEKAge < time.Duration(0) {
		return fmt.Errorf("--tek-age must be a positive duration, got %q", *allowedTEKAge)
	}
	if *symptomDayLimit < 0 {
		return fmt.Errorf("--symptom-days must be >=0, got %q", *symptomDayLimit)
	}
	if *fileAge < time.Duration(0) {
		return fmt.Errorf("--file-age must be a positive duration, got %q", *fileAge)
	}

	matches, err := filepath.Glob(*filePath)
	if err != nil {
		return fmt.Errorf("failed to expand matches: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("%q produced no matches (shell escaping?)", *filePath)
	}

	var errors *multierror.Error
	results := make([]*analysis, 0, len(matches))
	for _, m := range matches {
		result, err := analyzeOne(m, *showSig, *printJSON)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("%s: %w", m, err))
			continue
		}
		results = append(results, result)
	}

	for _, result := range results {
		printMsg("%s:\n", result.path)
		if !*quiet && len(result.sig) > 0 {
			printMsg("signature: %s", result.sig)
		}

		if !*quiet && len(result.export) > 0 {
			printMsg("export: %s", result.export)
		}
		printMsg("\n")
	}

	return errors.ErrorOrNil()
}

type analysis struct {
	path   string
	sig    []byte
	export []byte
}

func analyzeOne(pth string, includeSig, includeExport bool) (*analysis, error) {
	blob, err := ioutil.ReadFile(pth)
	if err != nil {
		return nil, fmt.Errorf("can't read export file: %w", err)
	}

	var result analysis
	result.path = pth

	if includeSig {
		sigExport, err := export.UnmarshalSignatureFile(blob)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling export signature file: %w", err)
		}

		prettyJSON, err := json.MarshalIndent(sigExport, "", " ")
		if err != nil {
			return nil, fmt.Errorf("error pretty printing export signature: %w", err)
		}
		result.sig = prettyJSON
	}

	keyExport, _, err := export.UnmarshalExportFile(blob)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling export file: %w", err)
	}

	// Do some basic data validation.
	if err := checkExportFile(keyExport); err != nil {
		return nil, fmt.Errorf("export file contains errors: %w", err)
	}

	if includeExport {
		prettyJSON, err := json.MarshalIndent(keyExport, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("error pretty printing export: %w", err)
		}
		result.export = prettyJSON
	}

	return &result, nil
}

func checkExportFile(export *exportpb.TemporaryExposureKeyExport) error {
	now := time.Now().UTC().Add(-1 * *fileAge)
	floor := model.IntervalNumber(now.Add(-1 * *allowedTEKAge))
	ceiling := model.IntervalNumber(now)

	var errors *multierror.Error
	if err := checkKeys("keys", export.Keys, floor, ceiling); err != nil {
		errors = multierror.Append(errors, err)
	}
	if err := checkKeys("revisedKeys", export.RevisedKeys, floor, ceiling); err != nil {
		errors = multierror.Append(errors, err)
	}
	return errors.ErrorOrNil()
}

func checkKeys(kType string, keys []*exportpb.TemporaryExposureKey, floor, ceiling int32) error {
	symptomDays := int32(*symptomDayLimit)
	var errors *multierror.Error
	for i, k := range keys {
		if l := len(k.KeyData); l != 16 {
			errors = multierror.Append(errors, fmt.Errorf("%s #%d: invald key length: want 16, got: %v", kType, i, l))
		}
		if s := k.GetRollingStartIntervalNumber(); s < floor {
			errors = multierror.Append(errors, fmt.Errorf("%s #%d: rolling interval start number is > %v ago, want >= %d, got %d", kType, i, *allowedTEKAge, floor, s))
		} else if s > ceiling {
			errors = multierror.Append(errors, fmt.Errorf("%s #%d: rolling interval start number in the future, want < %d, got %d", kType, i, ceiling, s))
		}
		if r := k.GetRollingPeriod(); r < 1 || r > 144 {
			errors = multierror.Append(errors, fmt.Errorf("%s #%d: rolling period invalid, want >= 1 && <= 144, got %d", kType, i, r))
		}
		if k.DaysSinceOnsetOfSymptoms != nil {
			if d := k.GetDaysSinceOnsetOfSymptoms(); d < -symptomDays || d > symptomDays {
				errors = multierror.Append(errors, fmt.Errorf("%s #%d: days_since_onset_of_symptoms is outside of expected range, -%d..%d, got: %d", kType, i, symptomDays, symptomDays, d))
			}
		}
	}
	return errors.ErrorOrNil()
}

func printMsg(msg string, args ...interface{}) {
	msg = fmt.Sprintf("%s\n", msg)
	fmt.Fprintf(os.Stdout, msg, args...)
}

func printError(msg string, args ...interface{}) {
	msg = fmt.Sprintf("ERROR! %s\n", msg)
	fmt.Fprintf(os.Stderr, msg, args...)
}
