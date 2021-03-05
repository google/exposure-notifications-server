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

package export

import (
	"crypto"
	"encoding/base64"
	"io"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
)

func TestMarshalUnmarshalExportFile(t *testing.T) {
	t.Parallel()

	batchStartTime := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).Truncate(time.Second)
	batchEndTime := batchStartTime.Add(1 * time.Hour)

	batch := &model.ExportBatch{
		BatchID:          1,
		ConfigID:         1,
		BucketName:       "test-bucket",
		FilenameRoot:     "files",
		StartTimestamp:   batchStartTime,
		EndTimestamp:     batchEndTime,
		OutputRegion:     "US",
		Status:           "",
		LeaseExpires:     time.Time{},
		SignatureInfoIDs: []int64{1, 2},
	}

	exposures := []*publishmodel.Exposure{
		{
			ExposureKey:           []byte("ABC"),
			Regions:               []string{"US"},
			IntervalNumber:        18,
			IntervalCount:         0,
			CreatedAt:             batchStartTime,
			LocalProvenance:       true,
			TransmissionRisk:      8,
			DaysSinceSymptomOnset: proto.Int32(0),
			ReportType:            verifyapi.ReportTypeClinical,
		},
		{
			ExposureKey:           []byte("DEF"),
			Regions:               []string{"CA"},
			IntervalNumber:        118,
			IntervalCount:         1,
			CreatedAt:             batchEndTime,
			LocalProvenance:       true,
			TransmissionRisk:      1,
			DaysSinceSymptomOnset: proto.Int32(-1),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
		{
			ExposureKey:           []byte("GHI"),
			Regions:               []string{"CA"},
			IntervalNumber:        118,
			IntervalCount:         1,
			CreatedAt:             batchEndTime,
			LocalProvenance:       true,
			TransmissionRisk:      5,
			DaysSinceSymptomOnset: proto.Int32(-1),
			ReportType:            verifyapi.ReportTypeSelfReport,
		},
	}
	revisedExposures := []*publishmodel.Exposure{
		{
			ExposureKey:           []byte("123"),
			Regions:               []string{"US"},
			IntervalNumber:        100,
			IntervalCount:         144,
			CreatedAt:             batchStartTime,
			LocalProvenance:       true,
			TransmissionRisk:      4,
			DaysSinceSymptomOnset: proto.Int32(2),
			RevisedReportType:     proto.String(verifyapi.ReportTypeNegative),
		},
	}

	signatureInfo := &model.SignatureInfo{
		SigningKey:        "/kms/project/key/1",
		SigningKeyVersion: "1",
		SigningKeyID:      "310",
		EndTimestamp:      batchEndTime,
	}

	signer := &customTestSigner{}

	blob, err := MarshalExportFile(batch, exposures, revisedExposures, int32(1), false /* single file batch */, []*Signer{
		{SignatureInfo: signatureInfo, Signer: signer},
	})
	if err != nil {
		t.Fatalf("Can't marshal export file, %v", err)
	}

	got, digest, err := UnmarshalExportFile(blob)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	wantDigest := "ho9AqUPleXoGs4wc5PEUU7MvbToDryEnQ+rJ8Fb1Hvc="
	if b64digest := base64.StdEncoding.EncodeToString(digest); b64digest != wantDigest {
		t.Errorf("wrong message digest want: %v, got: %v", wantDigest, b64digest)
	}

	infos := []*export.SignatureInfo{
		{
			VerificationKeyVersion: proto.String("1"),
			VerificationKeyId:      proto.String("310"),
			SignatureAlgorithm:     proto.String("1.2.840.10045.4.3.2"),
		},
	}

	keys := []*export.TemporaryExposureKey{
		{
			KeyData:                    []byte("ABC"),
			TransmissionRiskLevel:      proto.Int32(8),
			RollingStartIntervalNumber: proto.Int32(18),
			RollingPeriod:              proto.Int32(0),
			ReportType:                 export.TemporaryExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS.Enum(),
			DaysSinceOnsetOfSymptoms:   proto.Int32(0),
		},
		{
			KeyData:                    []byte("DEF"),
			TransmissionRiskLevel:      proto.Int32(1),
			RollingStartIntervalNumber: proto.Int32(118),
			RollingPeriod:              proto.Int32(1),
			ReportType:                 export.TemporaryExposureKey_CONFIRMED_TEST.Enum(),
			DaysSinceOnsetOfSymptoms:   proto.Int32(-1),
		},
		{
			KeyData:                    []byte("GHI"),
			TransmissionRiskLevel:      proto.Int32(5),
			RollingStartIntervalNumber: proto.Int32(118),
			RollingPeriod:              proto.Int32(1),
			ReportType:                 export.TemporaryExposureKey_SELF_REPORT.Enum(),
			DaysSinceOnsetOfSymptoms:   proto.Int32(-1),
		},
	}
	revisedKeys := []*export.TemporaryExposureKey{
		{
			KeyData:                    []byte("123"),
			TransmissionRiskLevel:      proto.Int32(4),
			RollingStartIntervalNumber: proto.Int32(100),
			ReportType:                 export.TemporaryExposureKey_REVOKED.Enum(),
		},
	}

	keyExport := &export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(uint64(batchStartTime.Unix())),
		EndTimestamp:   proto.Uint64(uint64(batchEndTime.Unix())),
		Region:         proto.String("US"),
		BatchNum:       proto.Int32(1),
		BatchSize:      proto.Int32(1),
		SignatureInfos: infos,
		Keys:           keys,
		RevisedKeys:    revisedKeys,
	}

	ignoredTemporaryExposureKeyExportFields := cmpopts.IgnoreUnexported(export.TemporaryExposureKeyExport{})
	ignoredSignatureInfoFields := cmpopts.IgnoreUnexported(export.SignatureInfo{})
	ignoredTemporaryExposureKeyFields := cmpopts.IgnoreUnexported(export.TemporaryExposureKey{})
	diff := cmp.Diff(keyExport, got, ignoredTemporaryExposureKeyExportFields, ignoredSignatureInfoFields, ignoredTemporaryExposureKeyFields)
	if diff != "" {
		t.Fatalf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}

type customTestSigner struct {
	sig []byte
	pub crypto.PublicKey
}

func (s *customTestSigner) Public() crypto.PublicKey { return s.pub }
func (s *customTestSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return s.sig, nil
}
