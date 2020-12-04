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

package mirror

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

type Config struct {
	Database              database.Config
	ObservabilityExporter observability.Config
	SecretManager         secrets.Config
	Storage               storage.Config

	Port string `env:"PORT, default=8080"`

	// Max file sizes for download. 1mb for index files, 20mb for zip files.
	MaxIndexBytes int64 `env:"MAX_INDEX_BYTES, default=1048576"`
	MaxZipBytes   int64 `env:"MAX_ZIP_BYTES, default=20971520"`

	// IndexFileDownloadTimeout is the amount of time to allow to download the
	// entire index file.
	IndexFileDownloadTimeout time.Duration `env:"INDEX_FILE_DOWNLOAD_TIMEOUT, default=30s"`

	// ExportFileDownloadTimeout, ExportFileDeleteTimeout, and
	// ExportFileUploadTimeout are the maximum amount of time to wait when
	// downloading, deleting, and uploading an export file, respectively.
	ExportFileDownloadTimeout time.Duration `env:"EXPORT_FILE_DOWNLOAD_TIMEOUT, default=1m"`
	ExportFileDeleteTimeout   time.Duration `env:"EXPORT_FILE_DELETE_TIMEOUT, default=10s"`
	ExportFileUploadTimeout   time.Duration `env:"EXPORT_FILE_UPLOAD_TIMEOUT, default=1m"`

	MaxRuntime         time.Duration `env:"MAX_RUNTIME, default=14m"`
	MirrorLockDuration time.Duration `env:"MIRROR_LOCK_DURATION, default=15m"`
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return &c.Storage
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}
