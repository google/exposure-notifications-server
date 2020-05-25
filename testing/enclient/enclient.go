package enclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/publish/model"
)

const (
	// httpTimeout is the maximum amount of time to wait for a response.
	httpTimeout = 30 * time.Second
)

type Interval int32

// Posts requests to the specified url.
// This methods attempts to serialize data argument as a json.
func PostRequest(url string, data interface{}) (*http.Response, error) {
	request := bytes.NewBuffer(JsonRequest(data))
	r, err := http.NewRequest("POST", url, request)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return error upstream.
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to copy error body (%d): %w", resp.StatusCode, err)
		}
		return resp, fmt.Errorf("post request failed with status: %v\n%v", resp.StatusCode, body)
	}

	return resp, nil
}

// Serializes the given argument to json.
func JsonRequest(data interface{}) []byte {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("unable to marshal json payload")
	}
	return jsonData
}

// Returns the Interval for the current moment of tme.
func NowInterval() Interval {
	return NewInterval(time.Now().Unix())
}

// Creates a new interval for the UNIX epoch given.
func NewInterval(time int64) Interval {
	return Interval(int32(time / 600))
}

// Creates an exposure key.
func ExposureKey(key string, intervalNumber Interval, intervalCount int32, transmissionRisk int) model.ExposureKey {
	return model.ExposureKey{
		Key:              key,
		IntervalNumber:   int32(intervalNumber),
		IntervalCount:    intervalCount,
		TransmissionRisk: transmissionRisk,
	}
}
