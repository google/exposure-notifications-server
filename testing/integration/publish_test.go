package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	export2 "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	keysInRequest    = 14
	transmissionRisk = 5
	region           = "TEST"
	exportDir        = "/tmp/en/exposureKeyExport-e2e"
)

func TestPublish(t *testing.T) {
	keys := util.GenerateExposureKeys(keysInRequest, transmissionRisk, true)
	request := publishRequest(keys, []string{region})

	publishKeys(t, request)

	t.Logf("Waiting before creating batches.")
	time.Sleep(1 * time.Minute)

	exportBatches(t)

	t.Logf("Waiting before starting export workers.")
	time.Sleep(5 * time.Second)

	startExportWorkers(t)

	got := collectExportResults(t, exportDir)

	wantedKeysMap := make(map[string]export2.TemporaryExposureKey)
	for _, key := range keys {
		wantedKeysMap[key.Key] = export2.TemporaryExposureKey{
			KeyData:                    util.DecodeKey(key.Key),
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
			RollingPeriod:              proto.Int32(key.IntervalCount),
		}
	}

	want := export2.TemporaryExposureKeyExport{
		StartTimestamp: nil,
		EndTimestamp:   nil,
		Region:         proto.String("TEST"),
		BatchNum:       proto.Int32(1),
		BatchSize:      proto.Int32(1),
		SignatureInfos: nil,
		Keys:           nil,
	}

	options := []cmp.Option{
		cmpopts.IgnoreFields(want, "StartTimestamp"),
		cmpopts.IgnoreFields(want, "EndTimestamp"),
		cmpopts.IgnoreFields(want, "SignatureInfos"),
		cmpopts.IgnoreFields(want, "Keys"),
		cmpopts.IgnoreUnexported(want),
	}

	diff := cmp.Diff(got, &want, options...)
	if diff != "" {
		t.Errorf("%v", diff)
	}

	for _, key := range got.Keys {
		s := util.ToBase64(key.KeyData)
		wantedKey := wantedKeysMap[s]
		gotKey := *key
		diff := cmp.Diff(wantedKey, gotKey, cmpopts.IgnoreUnexported(gotKey))
		if diff != "" {
			t.Errorf("invalid key value: %v:%v", s, diff)
		}
	}

	bytes, err := json.MarshalIndent(got, "", "")
	if err != nil {
		t.Fatalf("can't marshal json results: %v", err)
	}

	t.Logf("%v", string(bytes))
}
