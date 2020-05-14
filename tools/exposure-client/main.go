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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
)

const (
	// the length of a diagnosis key, always 16 bytes
	dkLen               = 16
	maxTransmissionRisk = 8
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

	exposureKeys := generateExposureKeys(*numKeys)

	transmissionRisk := *transmissionRiskFlag
	if transmissionRisk < 0 {
		transmissionRisk = randomInt(maxTransmissionRisk) + 1
	}

	regionIdx := randomInt(len(defaultRegions))
	region := defaultRegions[regionIdx]
	if *regions != "" {
		region = strings.Split(*regions, ",")
	}

	verificationAuthorityName := *authorityName
	if verificationAuthorityName == "" {
		verificationAuthorityName = randomArrValue(verificationAuthorityNames)
	}

	padding := make([]byte, randomInt(1000)+1000)
	_, err := rand.Read(padding)
	if err != nil {
		log.Printf("error generating padding: %v", err)
	}

	data := model.Publish{
		Keys:             exposureKeys,
		Regions:          region,
		AppPackageName:   *appPackage,
		TransmissionRisk: transmissionRisk,
		// This tool cannot generate valid safetynet attestations.
		DeviceVerificationPayload: "some invalid data",
		VerificationPayload:       verificationAuthorityName,
		Padding:                   base64.RawStdEncoding.EncodeToString(padding),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("unable to marshal json payload")
	}

	sendRequest(jsonData)

	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("Can't display JSON that was sent, error: %v", err)
	} else {
		log.Printf("payload: \n%v", string(prettyJSON))
	}

	if *twice {
		time.Sleep(1 * time.Second)
		log.Printf("sending the request again...")
		sendRequest(jsonData)
	}
}

func randIntervalCount() int32 {
	n, err := rand.Int(rand.Reader, big.NewInt(144))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int32(n.Int64() + 1) // valid values are 1-144
}

func randomInt(maxValue int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxValue)))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int(n.Int64())
}

func randomArrValue(arr []string) string {
	return arr[randomInt(len(arr))]
}

func generateExposureKeys(numKeys int) []model.ExposureKey {
	keys := make([][]byte, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = make([]byte, dkLen)
		_, err := rand.Read(keys[i])
		if err != nil {
			log.Fatalf("rand.Read: %v", err)
		}
	}
	// When publishing multiple keys - they'll be on different days.
	intervalCount := randIntervalCount()
	intervalNumber := int32(time.Now().Unix()/600) - intervalCount
	exposureKeys := make([]model.ExposureKey, numKeys)
	for i, rawKey := range keys {
		exposureKeys[i].Key = base64.StdEncoding.EncodeToString(rawKey)
		exposureKeys[i].IntervalNumber = intervalNumber
		exposureKeys[i].IntervalCount = intervalCount
		// Adjust interval math for next key.
		intervalCount = randIntervalCount()
		intervalNumber -= intervalCount
	}
	return exposureKeys
}

func sendRequest(jsonData []byte) {
	r, err := http.NewRequest("POST", *url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("error creating http request, %v", err)
	}
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		log.Fatalf("error on http request: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("response: %v", resp.Status)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failure response from server.")
	}
}
