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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/pb/export"

	"github.com/golang/protobuf/proto"
)

const (
	fixedHeaderWidth     = 16
	exportBinaryName     = "export.bin"
	exportSignatureName  = "export.sig"
	defaultIntervalCount = 144
	androidPackage       = "com.google.android.apps.exposurenotification"
	algorithm            = "ECDSA p-256 SHA-256"
)

func MarshalExportFile(eb *model.ExportBatch, exposures []*model.Exposure, batchNum, batchSize int) ([]byte, error) {
	// create main exposure key export binary
	expContents, err := marshalContents(eb, exposures, int32(batchNum), int32(batchSize))
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %w", err)
	}

	// create signature file
	sigContents, err := marshalSignature(expContents, int32(batchNum), int32(batchSize))
	if err != nil {
		return nil, fmt.Errorf("unable to marshal signature file: %w", err)
	}

	// create compressed archive of binary and signature
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	zf, err := zw.Create(exportBinaryName)
	if err != nil {
		return nil, fmt.Errorf("unable to create zip entry for export: %w", err)
	}
	_, err = zf.Write(expContents)
	if err != nil {
		return nil, fmt.Errorf("unable to write export to archive: %w", err)
	}
	zf, err = zw.Create(exportSignatureName)
	if err != nil {
		return nil, fmt.Errorf("unable to create zip entry for signature: %w", err)
	}
	_, err = zf.Write(sigContents)
	if err != nil {
		return nil, fmt.Errorf("unable to write signature to archive: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("unable to close archive: %w", err)
	}
	return buf.Bytes(), nil
}

func marshalContents(eb *model.ExportBatch, exposures []*model.Exposure, batchNum int32, batchSize int32) ([]byte, error) {
	exportBytes := []byte("EK Export v1    ")
	if len(exportBytes) != fixedHeaderWidth {
		return nil, fmt.Errorf("incorrect header length: %d", len(exportBytes))
	}
	// We want to scramble keys to ensure no associations, so arbitrarily sort them.
	// This could be done at the db layer but doing it here makes it explicit that its
	// important to the serialization
	sort.Slice(exposures, func(i, j int) bool {
		return bytes.Compare(exposures[i].ExposureKey, exposures[j].ExposureKey) < 0
	})
	var pbeks []*export.TemporaryExposureKey
	for _, exp := range exposures {
		pbek := export.TemporaryExposureKey{
			KeyData:               exp.ExposureKey,
			TransmissionRiskLevel: proto.Int32(int32(exp.TransmissionRisk)),
		}
		if exp.IntervalNumber != 0 {
			pbek.RollingStartIntervalNumber = proto.Int32(exp.IntervalNumber)
		}
		if exp.IntervalCount != defaultIntervalCount {
			pbek.RollingPeriod = proto.Int32(exp.IntervalCount)
		}
		pbeks = append(pbeks, &pbek)
	}
	pbeke := export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(uint64(eb.StartTimestamp.Unix())),
		EndTimestamp:   proto.Uint64(uint64(eb.EndTimestamp.Unix())),
		Region:         proto.String(eb.Region),
		BatchNum:       proto.Int32(int32(batchNum)),
		BatchSize:      proto.Int32(int32(batchSize)),
		Keys:           pbeks,
		SignatureInfos: []*export.SignatureInfo{
			{
				AndroidPackage: proto.String(androidPackage),
			},
		},
	}
	protoBytes, err := proto.Marshal(&pbeke)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %w", err)
	}
	return append(exportBytes, protoBytes...), nil
}

func marshalSignature(exportContents []byte, batchNum int32, batchSize int32) ([]byte, error) {
	sig, err := generateSignature(exportContents)
	if err != nil {
		return nil, fmt.Errorf("unable to generate signature: %v", err)
	}
	teks := &export.TEKSignature{
		SignatureInfo: &export.SignatureInfo{
			AndroidPackage:         proto.String(androidPackage),
			VerificationKeyVersion: proto.String("v1"),
			SignatureAlgorithm:     proto.String(algorithm),
		},
		BatchNum:  proto.Int32(batchNum),
		BatchSize: proto.Int32(batchSize),
		Signature: sig,
	}
	teksl := export.TEKSignatureList{
		Signatures: []*export.TEKSignature{teks},
	}
	protoBytes, err := proto.Marshal(&teksl)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal signature file: %v", err)
	}
	return protoBytes, nil
}

func getSigningKey() (*ecdsa.PrivateKey, error) {
	// TODO(guray): get from Cloud Key Store (or other kubernetes secrets storage)?
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func generateSignature(data []byte) ([]byte, error) {
	key, err := getSigningKey()
	if err != nil {
		return nil, fmt.Errorf("unable to generate signing key: %w", err)
	}
	digest := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("unable to sign: %w", err)
	}
	return elliptic.Marshal(elliptic.P256(), r, s), nil
}
