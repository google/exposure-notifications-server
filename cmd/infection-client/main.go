// This package is a CLI tool for generating test infection key data.
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
	"time"

	"cambio/pkg/model"
)

// the length of a diagnosis key, always 16 bytes
const dkLen = 16

var (
	url     = flag.String("url", "http://localhost:8080", "http(s) destination to send test record")
	numKeys = flag.Int("num", 1, "number of keys to generate -num=1")
)

func randIntervalCount() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(144))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return n.Int64() + 1 // valid values are 1-144
}

// This is a simple tester to call the infection API.
func main() {
	flag.Parse()

	keys := make([][]byte, *numKeys)
	for i := 0; i < *numKeys; i++ {
		keys[i] = make([]byte, dkLen)
		_, err := rand.Read(keys[i])
		if err != nil {
			log.Fatalf("rand.Read: %v", err)
		}
	}

	// When publishing multiple keys - they'll be on different days.
	intervalCount := randIntervalCount()
	intervalStart := time.Now().Unix()/600 - intervalCount

	diagnosisKeys := make([]model.DiagnosisKey, *numKeys)
	for i, rawKey := range keys {
		diagnosisKeys[i].Key = base64.StdEncoding.EncodeToString(rawKey)
		diagnosisKeys[i].IntervalStart = intervalStart
		diagnosisKeys[i].IntervalCount = intervalCount
		// Adjust interval math for next key.
		intervalCount = randIntervalCount()
		intervalStart -= intervalCount
	}

	// region settings for a key are assigned randomly
	regions := [][]string{
		[]string{"US"},
		[]string{"US", "CA"},
		[]string{"US", "CA", "MX"},
		[]string{"CA"},
		[]string{"CA", "MX"},
	}

	n, err := rand.Int(rand.Reader, big.NewInt(3))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	regionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(regions))))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}

	data := model.Publish{
		Keys:            diagnosisKeys,
		Regions:         regions[regionIdx.Int64()],
		AppPackageName:  "com.google.android",
		DiagnosisStatus: int(n.Int64()),
		Verification:    "",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("unable to marshal json payload")
	}

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
	log.Printf("wrote %v keys", len(keys))
	for _, key := range keys {
		log.Printf("  %v", key)
	}
}
