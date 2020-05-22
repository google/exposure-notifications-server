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

var (
	ExportBatchOpen     = "OPEN"
	ExportBatchPending  = "PENDING"
	ExportBatchComplete = "COMPLETE"
	ExportBatchDeleted  = "DELETED"
)

type ExportConfig struct {
	ConfigID         int64         `db:"config_id"`
	BucketName       string        `db:"bucket_name"`
	FilenameRoot     string        `db:"filename_root"`
	Period           time.Duration `db:"period_seconds"`
	Region           string        `db:"region"`
	From             time.Time     `db:"from_timestamp"`
	Thru             time.Time     `db:"thru_timestamp"`
	SignatureInfoIDs []int64       `db:"signature_info_ids"`
}

func (e *ExportConfig) FormattedFromTime() string {
	return e.From.Format(time.UnixDate)
}

func (e *ExportConfig) FormattedThruTime() string {
	if e.Thru.IsZero() {
		return ""
	}
	return e.Thru.Format(time.UnixDate)
}

type ExportBatch struct {
	BatchID          int64     `db:"batch_id" json:"batchID"`
	ConfigID         int64     `db:"config_id" json:"configID"`
	BucketName       string    `db:"bucket_name" json:"bucketName"`
	FilenameRoot     string    `db:"filename_root" json:"filenameRoot"`
	StartTimestamp   time.Time `db:"start_timestamp" json:"startTimestamp"`
	EndTimestamp     time.Time `db:"end_timestamp" json:"endTimestamp"`
	Region           string    `db:"region" json:"region"`
	Status           string    `db:"status" json:"status"`
	LeaseExpires     time.Time `db:"lease_expires" json:"leaseExpires"`
	SignatureInfoIDs []int64   `db:"signature_info_ids"`
}

type ExportFile struct {
	BucketName string `db:"bucket_name"`
	Filename   string `db:"filename"`
	BatchID    int64  `db:"batch_id"`
	Region     string `db:"region"`
	BatchNum   int    `db:"batch_num"`
	BatchSize  int    `db:"batch_size"`
	Status     string `db:"status"`
}

type SignatureInfo struct {
	ID                int64     `db:"id"`
	SigningKey        string    `db:"signing_key"`
	AppPackageName    string    `db:"app_package_name"`
	BundleID          string    `db:"bundle_id"`
	SigningKeyVersion string    `db:"signing_key_version"`
	SigningKeyID      string    `db:"signing_key_id"`
	EndTimestamp      time.Time `db:"thru_timestamp"`
}
