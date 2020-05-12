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

package federation

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
)

// Environment is the environment for the federation (data outbound) end point.
type Environment struct {
	Port     string        `envconfig:"PORT" default:"8080"`
	Timeout  time.Duration `envconfig:"RPC_TIMEOUT" default:"5m"`
	Database database.Environment

	// AllowAnyClient, if true, removes authentication requirements on the federation endpoint.
	// In practise, this is only useful in local testing.
	AllowAnyClient bool `envconfig:"ALLOW_ANY_CLIENT" default:"false"`

	// TLSCertFile is the certificate file to use if TLS encryption is enabled on the server.
	// If present, TLSKeyFile must also be present. These settings should be left blank on
	// Managed Cloud Run where the TLS termination is handled by the environment.
	TLSCertFile string `envconfig:"TLS_CERT_FILE"`
	TLSKeyFile  string `envconfig:"TLS_KEY_FILE"`
}
