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
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"sort"
	"time"

	"github.com/golang/protobuf/proto"
)

func MarshalExportFile(since, until time.Time, exposureKeys []*model.Infection, region string) ([]byte, error) {
	contents, err := marshalContents(since, until, exposureKeys, region)
	if err != nil {
		return nil, err
	}
	sig, err := sign(contents)
	if err != nil {
		return nil, err
	}
	return append(sig, contents...), nil
}

func marshalContents(since, until time.Time, exposureKeys []*model.Infection, region string) ([]byte, error) {
	// We want a deterministic ordering so signatures can be generated/verified consistently.
	// Arbitrarily sorting on the keys themselves.
	// This could be done at the db layer but doing it here makes it explicit that its
	// important to the serialization
	sort.Slice(exposureKeys, func(i, j int) bool {
		return bytes.Compare(exposureKeys[i].ExposureKey, exposureKeys[j].ExposureKey) < 0
	})
	var pbeks []*pb.ExposureKeyExport_ExposureKey
	for _, ek := range exposureKeys {
		pbek := pb.ExposureKeyExport_ExposureKey{
			ExposureKey:    ek.ExposureKey,
			IntervalNumber: ek.IntervalNumber,
			IntervalCount:  ek.IntervalCount,
		}
		pbeks = append(pbeks, &pbek)
	}
	batch := pb.ExposureKeyExport{
		StartTimestamp: since.Unix(),
		EndTimestamp:   until.Unix(),
		Region:         region,
		Keys:           pbeks,
	}
	return proto.Marshal(&batch)
}

func sign(contents []byte) ([]byte, error) {
	// TODO(guray): generate signature
	return []byte{}, nil
}
