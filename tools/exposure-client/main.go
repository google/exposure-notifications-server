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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
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
	if err := realMain(); err != nil {
		fmt.Printf("failed to create exposures: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("success!\n")
}

func realMain() error {
	flag.Parse()

	exposureKeys := util.GenerateExposureKeys(*numKeys, *transmissionRiskFlag, false)
	regionIdx, err := util.RandomInt(len(defaultRegions))
	if err != nil {
		return fmt.Errorf("failed to get random region: %w", err)
	}
	region := defaultRegions[regionIdx]
	if *regions != "" {
		region = strings.Split(*regions, ",")
	}

	verificationAuthorityName := *authorityName
	if verificationAuthorityName == "" {
		verificationAuthorityName, err = util.RandomArrValue(verificationAuthorityNames)
		if err != nil {
			return fmt.Errorf("failed to get random verification authority: %w", err)
		}
	}

	i, err := util.RandomInt(1000)
	if err != nil {
		return fmt.Errorf("failed to get random int: %w", err)
	}

	padding, err := util.RandomBytes(i + 1000)
	if err != nil {
		return fmt.Errorf("failed to get random padding: %w", err)
	}

	data := verifyapi.Publish{
		Keys:                exposureKeys,
		Regions:             region,
		AppPackageName:      *appPackage,
		VerificationPayload: verificationAuthorityName,
		Padding:             base64.RawStdEncoding.EncodeToString(padding),
	}

	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to generate JSON: %w", err)
	}
	fmt.Printf("generated json: \n%s\n", body)

	if _, err := sendRequest(bytes.NewReader(body)); err != nil {
		return fmt.Errorf("failed to send first request: %w", err)
	}

	if *twice {
		time.Sleep(1 * time.Second)
		if _, err := sendRequest(bytes.NewReader(body)); err != nil {
			return fmt.Errorf("failed to send second request: %w", err)
		}
	}

	return nil
}

func sendRequest(data io.Reader) ([]byte, error) {
	req, err := http.NewRequest("POST", *url, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("post request failed with status %v, body: %s", resp.StatusCode, body)
	}

	return body, nil
}
