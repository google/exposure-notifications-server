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

// Package server provides an opinionated http server.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/google/exposure-notifications-server/internal/logging"
	"go.opencensus.io/plugin/ochttp"
)

var (
	ErrAlreadyRunning = fmt.Errorf("already running")
)

// Server provides a gracefully-stoppable http server implementation. It is safe
// for concurrent use in goroutines.
type Server struct {
	addr    string
	handler http.Handler

	runLock sync.Mutex
	running bool
	srv     *http.Server
}

// New creates a new server listening on the provided port that responds to the
// http.Handler. It does not spawn or start the server.
func New(port string, handler http.Handler) *Server {
	return &Server{
		addr:    fmt.Sprintf(":%s", port),
		handler: handler,
	}
}

// Start starts the server. If no error is returned, the server is guaranteed to
// be listening when the function returns. Starting a running server is an
// error.
func (s *Server) Start(ctx context.Context) error {
	s.runLock.Lock()
	defer s.runLock.Unlock()

	if s.running {
		return ErrAlreadyRunning
	}

	// Create the net listener first, so the connection ready when we return. This
	// guarantees that it can accept requests.
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	// Create the server.
	s.srv = &http.Server{
		Handler: &ochttp.Handler{
			Handler: s.handler,
		},
	}

	// Start the server in the background. If there are any errors serving, log
	// them. Since this runs in a goroutine, there's no easy push these up.
	go func(ctx context.Context) {
		if err := s.srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger := logging.FromContext(ctx)
			logger.Errorf("http serving error: %v", err)
		}
	}(ctx)

	s.running = true
	return nil
}

// Stop terminates the server. The provided context can be given a timeout to
// limit the amount of time to wait for the server to start.
func (s *Server) Stop(ctx context.Context) error {
	s.runLock.Lock()
	defer s.runLock.Unlock()

	if !s.running {
		return nil
	}

	if err := s.srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to shutdown: %w", err)
	}

	s.running = false
	return nil
}
