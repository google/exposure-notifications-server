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

	"github.com/google/exposure-notifications-server/internal/export"
)

var (
	filePath = flag.String("file", "", "Path to the export zip file.")
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
		log.Fatalf("error unmarshalling export file: %v", err)
	}

	prettyJSON, err := json.MarshalIndent(keyExport, "", "  ")
	if err != nil {
		log.Fatalf("error pretty printing export: %v", err)
	}
	log.Printf("%v", string(prettyJSON))
}
