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
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/google/exposure-notifications-server/internal/export/model"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"

	"github.com/google/exposure-notifications-server/internal/pb/export"

	"google.golang.org/protobuf/proto"
)

const (
	fixedHeaderWidth     = 16
	exportBinaryName     = "export.bin"
	exportSignatureName  = "export.sig"
	defaultIntervalCount = 144
	// http://oid-info.com/get/1.2.840.10045.4.3.2
	algorithm = "1.2.840.10045.4.3.2"
)

type ExportSigners struct {
	SignatureInfo *model.SignatureInfo
	Signer        crypto.Signer
}

// MarshalExportFile converts the inputs into an encoded byte array.
func MarshalExportFile(eb *model.ExportBatch, exposures []*publishmodel.Exposure, batchNum, batchSize int, signers []ExportSigners) ([]byte, error) {
	// create main exposure key export binary
	expContents, err := marshalContents(eb, exposures, int32(batchNum), int32(batchSize), signers)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %w", err)
	}

	// create signature file
	sigContents, err := marshalSignature(eb, expContents, int32(batchNum), int32(batchSize), signers)
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

func marshalContents(eb *model.ExportBatch, exposures []*publishmodel.Exposure, batchNum int32, batchSize int32, signers []ExportSigners) ([]byte, error) {
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

	var exportSigInfos []*export.SignatureInfo
	for _, si := range signers {
		exportSigInfos = append(exportSigInfos, createSignatureInfo(si.SignatureInfo))
	}

	pbeke := export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(uint64(eb.StartTimestamp.Unix())),
		EndTimestamp:   proto.Uint64(uint64(eb.EndTimestamp.Unix())),
		Region:         proto.String(eb.Region),
		BatchNum:       proto.Int32(int32(batchNum)),
		BatchSize:      proto.Int32(int32(batchSize)),
		Keys:           pbeks,
		SignatureInfos: exportSigInfos,
	}
	protoBytes, err := proto.Marshal(&pbeke)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal exposure keys: %w", err)
	}
	return append(exportBytes, protoBytes...), nil
}

func createSignatureInfo(si *model.SignatureInfo) *export.SignatureInfo {
	sigInfo := &export.SignatureInfo{SignatureAlgorithm: proto.String(algorithm)}
	if si.AppPackageName != "" {
		sigInfo.AndroidPackage = proto.String(si.AppPackageName)
	}
	if si.BundleID != "" {
		sigInfo.AppBundleId = proto.String(si.BundleID)
	}
	if si.SigningKeyVersion != "" {
		sigInfo.VerificationKeyVersion = proto.String(si.SigningKeyVersion)
	}
	if si.SigningKeyID != "" {
		sigInfo.VerificationKeyId = proto.String(si.SigningKeyID)
	}
	return sigInfo
}

func marshalSignature(eb *model.ExportBatch, exportContents []byte, batchNum, batchSize int32, signers []ExportSigners) ([]byte, error) {

	var signatures []*export.TEKSignature
	for _, s := range signers {
		sig, err := generateSignature(exportContents, s.Signer)
		if err != nil {
			return nil, fmt.Errorf("unable to generate signature: %v", err)
		}
		teks := &export.TEKSignature{
			SignatureInfo: createSignatureInfo(s.SignatureInfo),
			BatchNum:      proto.Int32(batchNum),
			BatchSize:     proto.Int32(batchSize),
			Signature:     sig,
		}
		signatures = append(signatures, teks)
	}
	teksl := export.TEKSignatureList{
		Signatures: signatures,
	}
	protoBytes, err := proto.Marshal(&teksl)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal signature file: %v", err)
	}
	return protoBytes, nil
}

func generateSignature(data []byte, signer crypto.Signer) ([]byte, error) {
	digest := sha256.Sum256(data)
	sig, err := signer.Sign(rand.Reader, digest[:], nil)
	if err != nil {
		return nil, fmt.Errorf("unable to sign: %w", err)
	}
	return sig, nil
}
