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
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/google/exposure-notifications-server/internal/export/model"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"

	"github.com/google/exposure-notifications-server/internal/pb/export"

	"google.golang.org/protobuf/proto"
)

const (
	exportBinaryName     = "export.bin"
	exportSignatureName  = "export.sig"
	defaultIntervalCount = 144
	// http://oid-info.com/get/1.2.840.10045.4.3.2
	algorithm = "1.2.840.10045.4.3.2"
)

var (
	fixedHeader      = []byte("EK Export v1    ")
	fixedHeaderWidth = 16
)

type Signer struct {
	SignatureInfo *model.SignatureInfo
	Signer        crypto.Signer
}

// MarshalExportFile converts the inputs into an encoded byte array and sha256 (base64) of the marshaled protobuf contents.
func MarshalExportFile(eb *model.ExportBatch, exposures, revisedExposures []*publishmodel.Exposure, batchNum, batchSize int, signers []*Signer) ([]byte, string, error) {
	// create main exposure key export binary
	expContents, err := marshalContents(eb, exposures, revisedExposures, int32(batchNum), int32(batchSize), signers)
	if err != nil {
		return nil, "", fmt.Errorf("unable to marshal exposure keys: %w", err)
	}

	// create signature file
	sigContents, err := marshalSignature(expContents, int32(batchNum), int32(batchSize), signers)
	if err != nil {
		return nil, "", fmt.Errorf("unable to marshal signature file: %w", err)
	}

	// create compressed archive of binary and signature
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	zf, err := zw.Create(exportBinaryName)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create zip entry for export: %w", err)
	}
	_, err = zf.Write(expContents)
	if err != nil {
		return nil, "", fmt.Errorf("unable to write export to archive: %w", err)
	}
	zf, err = zw.Create(exportSignatureName)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create zip entry for signature: %w", err)
	}
	_, err = zf.Write(sigContents)
	if err != nil {
		return nil, "", fmt.Errorf("unable to write signature to archive: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("unable to close archive: %w", err)
	}

	digest := sha256.Sum256(expContents)
	pbSHA := hex.EncodeToString(digest[:])
	return buf.Bytes(), pbSHA, nil
}

// UnmarshalExportFile extracts the protobuf encoded exposure key present in the zip archived payload.
// Returns the parsed TemporaryExposureKeyExport protocol buffer message, the SHA256 digest of the signed content
// and/or an error if error.
// The digest is useful in validating the signature as it returns the deigest of the content that
// was signed when the archive was created.
func UnmarshalExportFile(zippedProtoPayload []byte) (*export.TemporaryExposureKeyExport, []byte, error) {
	zp, err := zip.NewReader(bytes.NewReader(zippedProtoPayload), int64(len(zippedProtoPayload)))
	if err != nil {
		return nil, nil, fmt.Errorf("can't read payload: %v", err)
	}

	for _, file := range zp.File {
		if file.Name == exportBinaryName {
			return unmarshalContent(file)
		}
	}

	return nil, nil, fmt.Errorf("payload is invalid: no %v file was found", exportBinaryName)
}

func unmarshalContent(file *zip.File) (*export.TemporaryExposureKeyExport, []byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}

	digest := sha256.Sum256(content)

	prefix := content[:fixedHeaderWidth]
	if !bytes.Equal(prefix, fixedHeader) {
		return nil, nil, fmt.Errorf("unknown prefix: %v", string(prefix))
	}

	message := new(export.TemporaryExposureKeyExport)
	err = proto.Unmarshal(content[fixedHeaderWidth:], message)
	if err != nil {
		return nil, nil, err
	}

	return message, digest[:], nil
}

func sortExposures(exposures []*publishmodel.Exposure) {
	sort.Slice(exposures, func(i, j int) bool {
		return bytes.Compare(exposures[i].ExposureKey, exposures[j].ExposureKey) < 0
	})
}

func makeTEK(exp *publishmodel.Exposure) *export.TemporaryExposureKey {
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
	return &pbek
}

func assignReportType(reportType *string, pbek *export.TemporaryExposureKey) {
	if reportType == nil {
		return
	}
	switch *reportType {
	case verifyapi.ReportTypeConfirmed:
		pbek.ReportType = export.TemporaryExposureKey_CONFIRMED_TEST.Enum()
	case verifyapi.ReportTypeClinical:
		pbek.ReportType = export.TemporaryExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS.Enum()
	case verifyapi.ReportTypeNegative:
		pbek.ReportType = export.TemporaryExposureKey_REVOKED.Enum()
	}
}

// The batch num and batch size are always set to 1 / 1 when actually generating a file.
// This is to avoid a device side issue where all batches must be passed in toegher.
// By making all batches size one, we make the unit of atomicity a single export file, avoiding
// the need for clients to do any kind of batching before passing to the OS.
//
// If this is part of a larger batch, the end timestamps are adjusted. Android hashes
// the start/end/batchNum to de-duplicate. If there are X files that have the same timing
// metadata, then only the first would get processed. We compensate here by bumping the end
// timestamp by the file num in the batch.
//
//
func marshalContents(eb *model.ExportBatch, exposures, revisedExposures []*publishmodel.Exposure, batchNum int32, batchSize int32, signers []*Signer) ([]byte, error) {
	exportBytes := fixedHeader
	if len(exportBytes) != fixedHeaderWidth {
		return nil, fmt.Errorf("incorrect header length: %d", len(exportBytes))
	}
	// We want to scramble keys to ensure no associations, so arbitrarily sort them.
	// This could be done at the db layer but doing it here makes it explicit that its
	// important to the serialization
	sortExposures(exposures)
	var pbeks []*export.TemporaryExposureKey
	for _, exp := range exposures {
		pbek := makeTEK(exp)
		assignReportType(&exp.ReportType, pbek)
		if exp.HasDaysSinceSymptomOnset() {
			pbek.DaysSinceOnsetOfSymptoms = proto.Int32(*exp.DaysSinceSymptomOnset)
		}
		pbeks = append(pbeks, pbek)
	}

	sortExposures(revisedExposures)
	var pbRevisedKeys []*export.TemporaryExposureKey
	for _, exp := range revisedExposures {
		pbek := makeTEK(exp)
		assignReportType(exp.RevisedReportType, pbek)
		pbek.DaysSinceOnsetOfSymptoms = exp.RevisedDaysSinceSymptomOnset
		pbRevisedKeys = append(pbRevisedKeys, pbek)
	}

	var exportSigInfos []*export.SignatureInfo
	for _, si := range signers {
		exportSigInfos = append(exportSigInfos, createSignatureInfo(si.SignatureInfo))
	}

	offset := int64(0)
	if batchSize > 1 {
		offset = int64(batchNum)
	}
	pbeke := export.TemporaryExposureKeyExport{
		StartTimestamp: proto.Uint64(uint64(eb.StartTimestamp.Unix())),
		EndTimestamp:   proto.Uint64(uint64(eb.EndTimestamp.Unix() + offset)),
		Region:         proto.String(eb.OutputRegion),
		BatchNum:       proto.Int32(1), // all batches are now size 1 (single file)
		BatchSize:      proto.Int32(1), // so it's always 1 of 1.
		Keys:           pbeks,
		RevisedKeys:    pbRevisedKeys,
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
	if si.SigningKeyVersion != "" {
		sigInfo.VerificationKeyVersion = proto.String(si.SigningKeyVersion)
	}
	if si.SigningKeyID != "" {
		sigInfo.VerificationKeyId = proto.String(si.SigningKeyID)
	}
	return sigInfo
}

// UnmarshalSignatureFile extracts the protobuf encode dsignatures.
func UnmarshalSignatureFile(zippedProtoPayload []byte) (*export.TEKSignatureList, error) {
	zp, err := zip.NewReader(bytes.NewReader(zippedProtoPayload), int64(len(zippedProtoPayload)))
	if err != nil {
		return nil, fmt.Errorf("can't read payload: %v", err)
	}

	for _, file := range zp.File {
		if file.Name == exportSignatureName {
			return unmarshalSignatureContent(file)
		}
	}

	return nil, fmt.Errorf("payload is invalid: no %v file was found", exportBinaryName)
}

func unmarshalSignatureContent(file *zip.File) (*export.TEKSignatureList, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	message := new(export.TEKSignatureList)
	err = proto.Unmarshal(content, message)
	if err != nil {
		return nil, err
	}

	return message, nil
}

func marshalSignature(exportContents []byte, batchNum, batchSize int32, signers []*Signer) ([]byte, error) {
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
	sig, err := signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("unable to sign: %w", err)
	}
	return sig, nil
}
