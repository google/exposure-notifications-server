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
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/pb/export"

	"github.com/golang/protobuf/proto"
)

const (
	fixedHeaderWidth     = 16
	exportBinaryName     = "export.bin"
	exportSignatureName  = "export.sig"
	defaultIntervalCount = 144
)

func MarshalExportFile(since, until time.Time, exposureKeys []*model.Exposure, region string, batchNum, batchSize int32) ([]byte, error) {
	// create main exposure key export binary
	expContents, err := marshalContents(since, until, exposureKeys, region, batchNum, batchSize)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %v", err)
	}

	// sign it
	sig, err := generateSignature(expContents)
	if err != nil {
		return nil, fmt.Errorf("unable to generate signature: %v", err)
	}

	// create compressed archive of binary and signature
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	zf, err := zw.Create(exportBinaryName)
	if err != nil {
		return nil, fmt.Errorf("unable to create zip entry for export: %v", err)
	}
	_, err = zf.Write(expContents)
	if err != nil {
		return nil, fmt.Errorf("unable to write export to archive: %v", err)
	}
	zf, err = zw.Create(exportSignatureName)
	if err != nil {
		return nil, fmt.Errorf("unable to create zip entry for signature: %v", err)
	}
	_, err = zf.Write(sig)
	if err != nil {
		return nil, fmt.Errorf("unable to write signature to archive: %v", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("unable to close archive: %v", err)
	}
	return buf.Bytes(), nil
}

func marshalContents(since, until time.Time, exposureKeys []*model.Exposure, region string, batchNum, batchSize int32) ([]byte, error) {
	exportBytes := []byte("EK Export v1    ")
	if len(exportBytes) != fixedHeaderWidth {
		return nil, fmt.Errorf("incorrect header length: %d", len(exportBytes))
	}
	// We want to scramble keys to ensure no associations, so arbitrarily sort them.
	// This could be done at the db layer but doing it here makes it explicit that its
	// important to the serialization
	sort.Slice(exposureKeys, func(i, j int) bool {
		return bytes.Compare(exposureKeys[i].ExposureKey, exposureKeys[j].ExposureKey) < 0
	})
	var pbeks []*export.TemporaryExposureKey
	for _, ek := range exposureKeys {
		pbek := export.TemporaryExposureKey{
			KeyData:               ek.ExposureKey,
			TransmissionRiskLevel: proto.Int32(int32(ek.TransmissionRisk)),
		}
		if ek.IntervalNumber != 0 {
			pbek.RollingStartIntervalNumber = proto.Int32(ek.IntervalNumber)
		}
		if ek.IntervalCount != defaultIntervalCount {
			pbek.RollingPeriod = proto.Int32(ek.IntervalCount)
		}
		pbeks = append(pbeks, &pbek)
	}
	pbeke := export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(uint64(since.Unix())),
		EndTimestamp:   proto.Uint64(uint64(until.Unix())),
		Region:         proto.String(region),
		BatchNum:       proto.Int32(batchNum),
		BatchSize:      proto.Int32(batchSize),
		Keys:           pbeks,
		// TODO(guray): SignatureInfos
	}
	protoBytes, err := proto.Marshal(&pbeke)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %v", err)
	}
	return append(exportBytes, protoBytes...), nil
}

func getSigningKey() (*rsa.PrivateKey, error) {
	// TODO(guray): get from Cloud Key Store (or other kubernetes secrets storage)?
	reader := rand.Reader
	bitSize := 2048
	return rsa.GenerateKey(reader, bitSize)
}

func generateSignature(data []byte) ([]byte, error) {
	key, err := getSigningKey()
	if err != nil {
		return nil, fmt.Errorf("unable to generate signing key: %v", err)
	}
	hash := sha256.New()
	hash.Write(data)
	digest := hash.Sum(nil)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
}
