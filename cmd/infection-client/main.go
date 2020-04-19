// This package is a CLI tool for generating test infection key data.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
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

	diagnosisKeys := make([]string, *numKeys)
	for i, rawKey := range keys {
		diagnosisKeys[i] = base64.StdEncoding.EncodeToString(rawKey)
	}

	data := model.Publish{
		Keys:           diagnosisKeys,
		AppPackageName: "com.google.android",
		Region:         []string{"US"},
		Platform:       "Android",
		Verification:   "",
		KeyDay:         time.Now().Unix(),
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
	log.Printf("key day: %v", data.KeyDay)
	log.Printf("wrote %v keys", len(keys))
	for _, key := range keys {
		log.Printf("  %v", key)
	}
}
