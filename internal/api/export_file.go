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

// Package api defines the structures for the infection publishing API.
package api

import (
	"bytes"
	"encoding/binary"
	"sort"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/pb"

	"github.com/golang/protobuf/proto"
)

func MarshalExportFile(since, until time.Time, exposureKeys []*model.Infection, region string) ([]byte, error) {
	// start with fixed width 16 byte header of basic file info and version
	exportBytes := []byte("EK Export v1   ")
	// We want a deterministic ordering so signatures can be generated/verified consistently.
	// Arbitrarily sorting on the keys themselves.
	// This could be done at the db layer but doing it here makes it explicit that its
	// important to the serialization
	sort.Slice(exposureKeys, func(i, j int) bool {
		return bytes.Compare(exposureKeys[i].ExposureKey, exposureKeys[j].ExposureKey) < 0
	})
	// Build up the various compact byte arrays from exposure keys
	var keys []byte
	var risks []byte
	var intervals []byte
	var rollings []byte
	for _, ek := range exposureKeys {
		keys = append(keys, ek.ExposureKey...)
		risks = append(risks, byte(ek.TransmissionRisk))

		interval, err := convert(ek.IntervalNumber)
		if err != nil {
			return nil, err
		}
		intervals = append(intervals, interval...)

		rolling, err := convert(ek.IntervalCount)
		if err != nil {
			return nil, err
		}
		rollings = append(rollings, rolling...)
	}
	m := pb.ExposureKeyExport{
		StartTimestamp:    proto.Uint64(uint64(time.Now().Unix())),
		EndTimestamp:      proto.Uint64(uint64(time.Now().Add(time.Hour * 24 * -1).Unix())),
		Region:            proto.String("US"),
		ExposureKeys:      keys,
		TransmissionRisks: risks,
		IntervalNumbers:   intervals,
		RollingPeriods:    rollings,
	}
	pBbytes, err := proto.Marshal(&m)
	if err != nil {
		panic("unable to marshal protobuf")
	}
	return append(exportBytes, pBbytes...), nil
}

func convert(i int32) ([]byte, error) {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.LittleEndian, uint16(i))
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}
