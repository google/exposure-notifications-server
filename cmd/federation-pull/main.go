// This package is the service that pulls federation results from federation partners; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

var (
	timeoutEnvVar  = "PULL_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	timeout := defaultTimeout
	if timeoutStr := os.Getenv(timeoutEnvVar); timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warnf("Failed to parse $%s value %q, using default.", timeoutEnvVar, timeoutStr)
			timeout = defaultTimeout
		}
	}
	logger.Infof("Using fetch timeout %v (override with $%s)", timeout, timeoutEnvVar)

	if err := database.Initialize(); err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/", api.HandleFederationPull(timeout))
	logger.Info("starting federation puller")
	log.Fatal(http.ListenAndServe(":8080", router))
}
