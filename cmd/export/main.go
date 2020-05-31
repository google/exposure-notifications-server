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

// This package is the service that publishes infected keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := realMain(); err != nil {
		logger.Fatal(err)
	}
}

func realMain() error {
	ctx := context.Background()

	config, env, closer, err := doSetup()
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	defer closer()

	server, err := NewServer(ctx, config, env)
	if err != nil {
		return fmt.Errorf("newserver: %w", err)
	}

	go server.Run()
	return nil
	// return server.Stop()
}

func doSetup() (*export.Config, *serverenv.ServerEnv, setup.Defer, error) {
	ctx := context.Background()

	var config export.Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		return nil, nil, nil, err
	}
	return &config, env, closer, err
}

type Server struct {
	ctx context.Context
	srv *http.Server
}

func NewServer(ctx context.Context, config *export.Config, env *serverenv.ServerEnv) (*Server, error) {
	exportServer, err := export.NewServer(config, env)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/create-batches", exportServer.CreateBatchesHandler)
	mux.HandleFunc("/do-work", exportServer.WorkerHandler)

	srv := &http.Server{
		Addr: ":" + config.Port,
		Handler: &ochttp.Handler{
			Handler: mux,
		},
	}

	return &Server{
		ctx: ctx,
		srv: srv,
	}, nil
}

// Run starts the server and blocks until stopped. For this reason, it is
// usually called via a goroutine.
func (s *Server) Run() {
	logger := logging.FromContext(s.ctx)
	logger.Infof("listening on %s", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("failed to run server: %v", err)
	}
}

func (s *Server) Stop() error {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}
	return nil
}
