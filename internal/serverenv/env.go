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

// Package serverenv defines common parameters for the sever environment.
package serverenv

import (
	"context"
	"os"

	"github.com/google/exposure-notifications-server/internal/logging"
)

const (
	portEnvVar  = "PORT"
	defaultPort = "8080"
)

type ServerEnv struct {
	port string
}

func New(ctx context.Context) *ServerEnv {
	env := &ServerEnv{
		port: defaultPort,
	}

	logger := logging.FromContext(ctx)

	if override := os.Getenv(portEnvVar); override != "" {
		env.port = override
	}
	logger.Info("using port %v (override with $%v)", env.port, portEnvVar)

	return env
}

func (s *ServerEnv) Port() string {
	return s.port
}
