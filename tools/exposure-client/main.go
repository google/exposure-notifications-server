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

// This package is a CLI tool for generating test exposure key data.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/testing/enclient"
)

var (
	url                  = flag.String("url", "http://localhost:8080", "http(s) destination to send test record")
	numKeys              = flag.Int("num", 1, "number of keys to generate -num=1")
	twice                = flag.Bool("twice", false, "send the same request twice w/ delay")
	appPackage           = flag.String("app", "com.example.android.app", "AppPackageName to use in request")
	regions              = flag.String("regions", "", "Comma separated region names")
	authorityName        = flag.String("authority", "", "Verification Authority Name")
	transmissionRiskFlag = flag.Int("transmissionRisk", -1, "Transmission risk")
	// region settings for a key are assigned randomly
	defaultRegions = [][]string{
		{"US"},
		{"US", "CA"},
		{"US", "CA", "MX"},
		{"CA"},
		{"CA", "MX"},
	}

	// verificationAuth for a key are assigned randomly
	verificationAuthorityNames = []string{
		"",
		"AAA Health",
		"BBB Labs",
	}
)

func main() {
	flag.Parse()

	exposureKeys := enclient.GenerateExposureKeys(*numKeys, *transmissionRiskFlag)
	regionIdx := randomInt(len(defaultRegions))
	region := defaultRegions[regionIdx]
	if *regions != "" {
		region = strings.Split(*regions, ",")
	}

	verificationAuthorityName := *authorityName
	if verificationAuthorityName == "" {
		verificationAuthorityName = randomArrValue(verificationAuthorityNames)
	}

	padding := enclient.RandomBytes(randomInt(1000) + 1000)

	data := database.Publish{
		Keys:           exposureKeys,
		Regions:        region,
		AppPackageName: *appPackage,
		// This tool cannot generate valid safetynet attestations.
		DeviceVerificationPayload: "some invalid data",
		VerificationPayload:       verificationAuthorityName,
		Padding:                   base64.RawStdEncoding.EncodeToString(padding),
	}

	sendRequest(data)

	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("Can't display JSON that was sent, error: %v", err)
	} else {
		log.Printf("payload: \n%v", string(prettyJSON))
	}

	if *twice {
		time.Sleep(1 * time.Second)
		log.Printf("sending the request again...")
		sendRequest(data)
	}
}

func randomInt(maxValue int) int {
	return enclient.RandomInt(maxValue)
}

func randomArrValue(arr []string) string {
	return arr[randomInt(len(arr))]
}

func sendRequest(data interface{}) {
	resp, err := enclient.PostRequest(*url, data)
	if err != nil {
		log.Fatalf("request failed: %v, %v", err, resp)
		return
	}

	log.Printf("response: %v", resp.Status)
}
