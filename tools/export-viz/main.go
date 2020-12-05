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

// This tool writes a visualization of an export file.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"

	"github.com/google/exposure-notifications-server/internal/export"
	exportpb "github.com/google/exposure-notifications-server/internal/pb/export"
)

var (
	filePath = flag.String("file", "", "path to the export file")
)

// Example usage - requires graphviz
//
// go run ./tools/export-viz --file export.zip > graph
// dot -Tsvg graph > graph.svg
//
// Open the SVG file with a viewer (i.e. Google Chrome)
func main() {
	if err := realMain(); err != nil {
		log.Printf("ERROR: %v", err)
		os.Exit(1)
	}
}

func realMain() error {
	flag.Parse()
	if *filePath == "" {
		return fmt.Errorf("--file is required")
	}

	blob, err := ioutil.ReadFile(*filePath)
	if err != nil {
		return fmt.Errorf("can't read export file: %w", err)
	}

	keyExport, _, err := export.UnmarshalExportFile(blob)
	if err != nil {
		return err
	}

	// Build all of the nodes and map them to days (TEK start)
	nodeMap := make(map[string]string)
	nodes := make([]string, 0, len(keyExport.Keys))
	startIntervals := make([]int32, 0, 14)
	days := make(map[int32][]*exportpb.TemporaryExposureKey, len(keyExport.Keys))
	for i, k := range keyExport.Keys {
		start := *k.RollingStartIntervalNumber
		dayKeys, ok := days[start]
		if !ok {
			dayKeys = make([]*exportpb.TemporaryExposureKey, 0, 1)
			days[start] = dayKeys
			startIntervals = append(startIntervals, start)
		}

		days[start] = append(dayKeys, k)
		nodeName := fmt.Sprintf("n%d", i)
		nodeMap[base64.StdEncoding.EncodeToString(k.KeyData)] = nodeName
		nodes = append(nodes, nodeName)
	}

	buf := bytes.NewBufferString("")
	buf.WriteString("digraph regexp {\n")
	for _, v := range nodes {
		buf.WriteString(fmt.Sprintf(" %s;\n", v))
	}

	// Sort the days that we want to process (start intervals).
	sort.Slice(startIntervals, func(i, j int) bool { return startIntervals[i] < startIntervals[j] })

	// For through the start intervals and build potential links.
	for _, si := range startIntervals {
		nextStart := si + 144
		teks, ok := days[si]
		if !ok {
			return fmt.Errorf("start interval has no TEKs")
		}

		nextTeks, ok := days[nextStart]
		if !ok {
			// last day
			continue
		}

		for _, tek := range teks {
			if tek.RollingPeriod != nil && *tek.RollingPeriod < 144 {
				// this is a same day tek - won't have a next day one.
				continue
			}
			tekNode := nodeMap[base64.StdEncoding.EncodeToString(tek.KeyData)]
			for _, next := range nextTeks {
				// Could tek possibly come before next
				link := sameReportType(tek, next) &&
					sameTransmissionRisk(tek, next) &&
					linearSymptomOnset(tek, next)
				if link {
					nextNode := nodeMap[base64.StdEncoding.EncodeToString(next.KeyData)]
					if tekNode == nextNode {
						continue
					}
					buf.WriteString(fmt.Sprintf(" %s -> %s;\n", tekNode, nextNode))
				}
			}
		}
	}

	buf.WriteString("}\n")
	fmt.Printf("%s", buf.String())
	return nil
}

func sameReportType(a, b *exportpb.TemporaryExposureKey) bool {
	return a.GetReportType() == b.GetReportType()
}

func sameTransmissionRisk(a, b *exportpb.TemporaryExposureKey) bool {
	//lint:ignore SA1019 may be set on v1 files.
	return a.GetTransmissionRiskLevel() == b.GetTransmissionRiskLevel()
}

func linearSymptomOnset(a, b *exportpb.TemporaryExposureKey) bool {
	return (a.DaysSinceOnsetOfSymptoms == nil && b.DaysSinceOnsetOfSymptoms == nil) ||
		(a.GetDaysSinceOnsetOfSymptoms() == 10 && b.GetDaysSinceOnsetOfSymptoms() == 10) ||
		a.GetDaysSinceOnsetOfSymptoms()+1 == b.GetDaysSinceOnsetOfSymptoms()
}
