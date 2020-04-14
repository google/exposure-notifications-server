package main

import (
	"bytes"
	"cambio/pkg/model"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"
)

// This is a simple tester to call the infection API.
func main() {
	var url = flag.String("url", "http://localhost:8080", "http(s) destination to send test record")
	flag.Parse()

	diagnosisKey := make([]byte, 16)
	for i := range diagnosisKey {
		diagnosisKey[i] = 42
	}

	data := model.Publish{
		Keys:           []string{base64.StdEncoding.EncodeToString(diagnosisKey)},
		AppPackageName: "com.google.android",
		Country:        "US",
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
}
