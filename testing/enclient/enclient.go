package enclient

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
)

const (
	dkLen               = 16
	maxTransmissionRisk = 8
	maxIntervals        = 144
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

// Generates random exposure keys.
// numKeys - number of keys to generate.
// transmissionRisk - transmission risk to use.
func GenerateExposureKeys(numKeys int, transmissionRisk int) []database.ExposureKey {
	// When publishing multiple keys - they'll be on different days.
	intervalCount := randIntervalCount()
	intervalNumber := NowInterval() - Interval(intervalCount)
	exposureKeys := make([]database.ExposureKey, numKeys)
	for i := 0; i < numKeys; i++ {
		tr := transmissionRisk
		if tr < 0 {
			tr = RandomInt(maxTransmissionRisk) + 1
		}

		exposureKeys[i] = RandExposureKey(intervalNumber, intervalCount, tr)
		// Adjust interval math for next key.
		intervalCount = randIntervalCount()
		intervalNumber -= Interval(intervalCount)
	}
	return exposureKeys
}

// Returns the Interval for the current moment of tme.
func NowInterval() Interval {
	return NewInterval(time.Now().Unix())
}

// Creates a new interval for the UNIX epoch given.
func NewInterval(time int64) Interval {
	return Interval(int32(time / 600))
}

// Creates a random exposure key.
func RandExposureKey(intervalNumber Interval, intervalCount int32, transmissionRisk int) database.ExposureKey {
	return ExposureKey(generateKey(), intervalNumber, intervalCount, transmissionRisk)
}

// Creates an exposure key.
func ExposureKey(key string, intervalNumber Interval, intervalCount int32, transmissionRisk int) database.ExposureKey {
	return database.ExposureKey{
		Key:              key,
		IntervalNumber:   int32(intervalNumber),
		IntervalCount:    intervalCount,
		TransmissionRisk: transmissionRisk,
	}
}

// Generates the random byte sequence.
func RandomBytes(arrLen int) []byte {
	padding := make([]byte, arrLen)
	_, err := rand.Read(padding)
	if err != nil {
		log.Fatalf("error generating padding: %v", err)
	}
	return padding
}

// Return the random int value.
func RandomInt(maxValue int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxValue)))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int(n.Int64())
}

// Returns the random interval count.
func randIntervalCount() int32 {
	n, err := rand.Int(rand.Reader, big.NewInt(maxIntervals))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int32(n.Int64() + 1) // valid values are 1-144
}

func generateKey() string {
	return ToBase64(RandomBytes(dkLen))
}

// Encodes bytes array to base64.
func ToBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// Decodes base64 string to []byte.
func DecodeKey(b64key string) []byte {
	k, err := base64.StdEncoding.DecodeString(b64key)
	if err != nil {
		log.Fatalf("unable to decode key: %v", err)
	}
	return k
}
