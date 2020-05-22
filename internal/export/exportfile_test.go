package export

import (
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/stretchr/testify/assert"
)

const sampleFilename = "../../examples/export/testExport-2-records-1-of-1.zip"

func TestUnmarshalExportFile(t *testing.T) {

	bytes, e := ioutil.ReadFile(sampleFilename)
	if e != nil {
		t.Errorf("Can't read sample file %v: %w", sampleFilename, e)
	}

	infos := []*export.SignatureInfo{
		{
			VerificationKeyVersion: proto.String("1"),
			VerificationKeyId:      proto.String("some_id"),
			SignatureAlgorithm:     proto.String("1.2.840.10045.4.3.2"),
		},
	}

	keys := []*export.TemporaryExposureKey{
		{
			KeyData:                    []byte("\x17/Ā\xe5\x98I\x0c4\x17v\x043\x9b\x15C"),
			TransmissionRiskLevel:      proto.Int32(8),
			RollingStartIntervalNumber: proto.Int32(2649980),
			RollingPeriod:              proto.Int32(1),
		},
		{
			KeyData:                    []byte("+c\x05:ă#\x8d\xe1v\x16\x9e\x86\x1f=\xcd"),
			TransmissionRiskLevel:      proto.Int32(1),
			RollingStartIntervalNumber: proto.Int32(2649866),
			RollingPeriod:              proto.Int32(114),
		},
	}

	keyExport := &export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(1588345200),
		EndTimestamp:   proto.Uint64(1588431600),
		Region:         proto.String("US"),
		BatchNum:       proto.Int32(1),
		BatchSize:      proto.Int32(1),
		SignatureInfos: infos,
		Keys:           keys,
	}

	got, err := UnmarshalExportFile(bytes)
	if err != nil {
		t.Errorf("Unmarshal failed: %w", err)
	}

	assert.Equal(t, keyExport.StartTimestamp, got.StartTimestamp)
	assert.Equal(t, keyExport.EndTimestamp, got.EndTimestamp)
	assert.Equal(t, keyExport.Region, got.Region)
	assert.Equal(t, keyExport.BatchNum, got.BatchNum)
	assert.Equal(t, keyExport.BatchSize, got.BatchSize)
	assert.Equal(t, keyExport.SignatureInfos, got.SignatureInfos)
	assert.Equal(t, keyExport.Keys, got.Keys)
}
