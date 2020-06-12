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

package publish

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/handlers"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifydb "github.com/google/exposure-notifications-server/internal/verification/database"
)

// Server hosts end points to manage export batches.
type Server struct {
	config      *Config
	env         *serverenv.ServerEnv
	transformer *publishmodel.Transformer
	verifier    *verification.Verifier
}

// NewServer makes a Server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	// Validate config.
	if env.Database() == nil {
		return nil, fmt.Errorf("missing Database provider in server env")
	}
	if env.AuthorizedAppProvider() == nil {
		return nil, fmt.Errorf("missing AuthorizedApp provider in server env")
	}

	transformer, err := publishmodel.NewTransformer(config.MaxKeysOnPublish, config.MaxIntervalAge, config.TruncateWindow, config.DebugReleaseSameDayKeys)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}

	verifier, err := verification.New(verifydb.New(env.Database()), &config.Verification)
	if err != nil {
		return nil, fmt.Errorf("verification.New: %w", err)
	}

	return &Server{
		config:      config,
		env:         env,
		transformer: transformer,
		verifier:    verifier,
	}, nil
}

func (s *Server) Routes(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()

	// There is a target normalized latency for this function. This is to help
	// prevent clients from being able to distinguish from successful or errored
	// requests.
	latency := s.config.MinRequestDuration
	h := s.handleError(s.handlePublish(ctx))
	h = handlers.WithMinimumLatency(latency, h)
	mux.HandleFunc("/", h)

	return mux
}
