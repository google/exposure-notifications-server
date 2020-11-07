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

package debugger

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/server"
)

// Server is the debugger server.
type Server struct {
	config *Config
	env    *serverenv.ServerEnv
}

// NewServer makes a new debugger server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.AuthorizedAppProvider() == nil {
		return nil, fmt.Errorf("missing AuthorizedAppProvider in server env")
	}
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("missing Blobstore in server env")
	}
	if env.Database() == nil {
		return nil, fmt.Errorf("missing Database in server env")
	}
	if env.KeyManager() == nil {
		return nil, fmt.Errorf("missing KeyManager in server env")
	}
	if env.SecretManager() == nil {
		return nil, fmt.Errorf("missing SecretManager in server env")
	}

	return &Server{
		config: config,
		env:    env,
	}, nil
}

func (s *Server) Routes(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDebug(ctx))
	mux.Handle("/health", server.HandleHealthz)

	return mux
}
