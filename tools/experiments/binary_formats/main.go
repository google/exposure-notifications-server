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

package main

/*

Quick utility to experiment with file sizes with different formats, # of keys, and compression ratios.

For 30K new cases a day, the file size would be:
 - grouped by key: 8.5MB (7.4MB compressed, 1.15 compression ratio)
 - transposed by field: 8.5MB (6.7MB, 1.27 compression ratio)
 - protobuf: 12MB (7.6MB compressed, 1.58 compression ratio)
 - flatbuffer: 16MB (8.2MB compressed, 1.95 compression ratio)
 - series of protobufs of just exposure keys: 11MB (7.4MB compressed, 1.49 compression ratio)
*/

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"io"
	mrand "math/rand"
	"os"

	"github.com/googlepartners/exposure-notifications/internal/pb"
	"github.com/googlepartners/exposure-notifications/tools/experiments/binary_formats/flat_exp"

	"github.com/golang/protobuf/proto"
	flatbuffers "github.com/google/flatbuffers/go"
)

const (
	filenameTrans    = "/tmp/testBinaryTransposed.bin"
	filenameGrp      = "/tmp/testBinaryGrp.bin"
	filenamePb       = "/tmp/testBinaryPb.bin"
	filenameFb       = "/tmp/testBinaryFb.bin"
	filenamePbs      = "/tmp/testBinaryPbs.bin"
	numKeys          = 30000 * 14 // 30K new cases per day in U.S. (100K globally), times 14 daily keys uploaded
	exposureKeyBytes = 16
	periodDays       = 14
	intervalsPerDay  = 144
)

func main() {
	// write transposed file
	fT, err := os.Create(filenameTrans)
	if err != nil {
		panic("Cannot open output file")
	}
	wT := bufio.NewWriter(fT)
	for i := 0; i < numKeys; i++ {
		writeKey(wT)
	}
	for i := 0; i < numKeys; i++ {
		writeInterval(wT)
	}
	for i := 0; i < numKeys; i++ {
		writeRollingPeriod(wT)
	}
	for i := 0; i < numKeys; i++ {
		writeTransmissionRisk(wT)
	}
	if err = wT.Flush(); err != nil {
		panic("couldn't flush")
	}
	if err = fT.Close(); err != nil {
		panic("couldn't close")
	}

	// write grouped file
	fG, err := os.Create(filenameGrp)
	if err != nil {
		panic("cannot open group output file")
	}
	wG := bufio.NewWriter(fG)
	for i := 0; i < numKeys; i++ {
		writeKey(wG)
		writeInterval(wG)
		writeRollingPeriod(wG)
		writeTransmissionRisk(wG)
	}
	if err = wG.Flush(); err != nil {
		panic("couldn't flush")
	}
	if err = fG.Close(); err != nil {
		panic("couldn't close")
	}

	// write pb file
	fP, err := os.Create(filenamePb)
	if err != nil {
		panic("cannot open pb output file")
	}
	wP := bufio.NewWriter(fP)
	var pbeks []*pb.ExposureKeyExport_ExposureKey
	for i := 0; i < numKeys; i++ {
		pbek := pb.ExposureKeyExport_ExposureKey{
			ExposureKey:      getRandKey(),
			IntervalNumber:   int32(getRandInterval()),
			IntervalCount:    int32(getRollingPeriod()),
			TransmissionRisk: int32(getTransmissionRisk()),
		}
		pbeks = append(pbeks, &pbek)
	}
	batch := pb.ExposureKeyExport{
		Keys: pbeks,
	}
	data, err := proto.Marshal(&batch)
	_, err = wP.Write(data)
	if err != nil {
		panic("did not write all bytes")
	}
	if err = wP.Flush(); err != nil {
		panic("couldn't flush")
	}
	if err = fP.Close(); err != nil {
		panic("couldn't close")
	}

	// write series of serialized pb ExposureKeys only
	fPB, err := os.Create(filenamePbs)
	if err != nil {
		panic("cannot open pbs output file")
	}
	wPB := bufio.NewWriter(fPB)
	for i := 0; i < numKeys; i++ {
		pbek := pb.ExposureKeyExport_ExposureKey{
			ExposureKey:      getRandKey(),
			IntervalNumber:   int32(getRandInterval()),
			IntervalCount:    int32(getRollingPeriod()),
			TransmissionRisk: int32(getTransmissionRisk()),
		}
		data, err := proto.Marshal(&pbek)
		if err != nil {
			panic("problem marshalling pbs")
		}
		_, err = wPB.Write(data)
		if err != nil {
			panic("didn't write all bytes")
		}
	}
	if err = wPB.Flush(); err != nil {
		panic("couldn't flush")
	}
	if err = fPB.Close(); err != nil {
		panic("couldn't close")
	}

	// write flatbuffer file
	fF, err := os.Create(filenameFb)
	if err != nil {
		panic("cannot open fb output file")
	}
	wF := bufio.NewWriter(fF)
	builder := flatbuffers.NewBuilder(1024)
	var keyOffsets [numKeys]flatbuffers.UOffsetT
	for i := 0; i < numKeys; i++ {
		flat_exp.ExposureKeyStartExposureKeyVector(builder, exposureKeyBytes)
		for j := 0; j < exposureKeyBytes; j++ {
			rk := getRandKey()
			builder.PrependByte(rk[j])
		}
		keyBytes := builder.EndVector(exposureKeyBytes)
		flat_exp.ExposureKeyStart(builder)
		flat_exp.ExposureKeyAddExposureKey(builder, keyBytes)
		flat_exp.ExposureKeyAddIntervalNumber(builder, int32(getRandInterval()))
		flat_exp.ExposureKeyAddIntervalCount(builder, int32(getRollingPeriod()))
		flat_exp.ExposureKeyAddTransmissionRisk(builder, int32(getTransmissionRisk()))
		keyOffsets[i] = flat_exp.ExposureKeyEnd(builder)
	}
	flat_exp.ExposureKeyExportStart(builder)
	exp := flat_exp.ExposureKeyExportEnd(builder)
	builder.Finish(exp)
	buf := builder.FinishedBytes()
	_, err = wF.Write(buf)
	if err != nil {
		panic("couldn't write all bytes")
	}
	if err = wF.Flush(); err != nil {
		panic("couldn't flush")
	}
	if err = fF.Close(); err != nil {
		panic("couldn't close")
	}
}

// random 128 bit exposure keys
func getRandKey() []byte {
	key := make([]byte, exposureKeyBytes)
	_, err := rand.Read(key)
	if err != nil {
		panic("couldn't create random key")
	}
	return key
}

func writeKey(w io.Writer) {
	_, err := w.Write(getRandKey())
	if err != nil {
		panic("couldn't write")
	}
}

// 13/14 chance of being 144
func getRandInterval() int {
	intervalNum := 144
	if mrand.Intn(periodDays) == 0 {
		intervalNum = mrand.Intn(intervalsPerDay)
	}
	return intervalNum
}

// can't use uint8 if we later want to go to 5 minutes so uint16
func writeInterval(w io.Writer) {
	err := binary.Write(w, binary.LittleEndian, uint16(getRandInterval()))
	if err != nil {
		panic("problem with interval number")
	}
}

// always 144 at launch of system
func getRollingPeriod() int {
	return 144
}

// can't use uint8 if we later want to go to 5 minutes, so uint16
func writeRollingPeriod(w io.Writer) {
	err := binary.Write(w, binary.LittleEndian, uint16(getRollingPeriod()))
	if err != nil {
		panic("problem with rolling period")
	}
}

// uniformly random transmission risk (8 values)
func getTransmissionRisk() int {
	return mrand.Intn(8)
}

func writeTransmissionRisk(w io.Writer) {
	err := binary.Write(w, binary.LittleEndian, uint8(getTransmissionRisk()))
	if err != nil {
		panic("problem with transmission risk")
	}
}
