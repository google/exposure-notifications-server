package integration

import (
	"log"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/testing/enclient"
)

const appPackageName = ""

func TestCleanupAPI(t *testing.T) {
	var bts []byte
	requestUrl := "http://localhost:8080/cleanup-exposure"
	resp, err := enclient.PostRequest(requestUrl, bts)
	if err != nil {
		t.Errorf("request failed: %v, %v", err, resp)
		return
	}

	log.Printf("response: %v", resp.Status)
	t.Logf("Cleanup request is sent to %v", requestUrl)
}

func TestExportAPI(t *testing.T) {
	var bts []byte
	requestUrl := "http://localhost:8080/export/create-batches"
	resp, err := enclient.PostRequest(requestUrl, bts)
	if err != nil {
		t.Errorf("request failed: %v, %v", err, resp)
		return
	}

	log.Printf("response: %v", resp.Status)
	t.Logf("Create batches request is sent to %v", requestUrl)
}

func TestExportWorkerApi(t *testing.T) {
	var bts []byte
	requestUrl := "http://localhost:8080/export/do-work"
	resp, err := enclient.PostRequest(requestUrl, bts)
	if err != nil {
		t.Errorf("request failed: %v, %v", err, resp)
		return
	}

	log.Printf("response: %v", resp.Status)
	t.Logf("Export worker request is sent to %v", requestUrl)
}

func publishKey(keys []database.ExposureKey, regions []string) database.Publish {
	padding := enclient.RandomBytes(1000)
	return database.Publish{
		Keys:                      keys,
		Regions:                   regions,
		AppPackageName:            appPackageName,
		DeviceVerificationPayload: "Test Device Verification Payload",
		VerificationPayload:       "Test Authority",
		Padding:                   enclient.ToBase64(padding),
	}
}
