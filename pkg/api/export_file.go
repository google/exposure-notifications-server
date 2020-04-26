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
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"time"

	"github.com/golang/protobuf/proto"
)

func MarshalExportFile(since, until time.Time, exposureKeys []*model.Infection, region string) ([]byte, error) {
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
