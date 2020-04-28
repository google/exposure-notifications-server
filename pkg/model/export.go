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

package model

import (
	"time"
)

type ExportBatch struct {
	BatchId        int       `db:"batch_id"`
	StartTimestamp time.Time `db:"start_timestamp"`
	EndTimestamp   time.Time `db:"end_timestamp"`
	Status         string    `db:"status"`
}

type ExportFile struct {
	Filename  string `db:"filename"`
	BatchId   int    `db:"batch_id"`
	Region    string `db:"region"`
	BatchNum  int    `db:"batch_num"`
	BatchSize int    `db:"batch_size"`
	Status    string `db:"status"`
}
