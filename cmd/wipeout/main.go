// This package is the service that deletes old infection keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"

	"github.com/gorilla/mux"
)

const (
	timeoutEnvVar  = "WIPEOUT_TIMEOUT"
	defaultTimeout = 10 * time.Minute
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
	logger.Infof("Using timeout %v (override with $%s)", timeout, timeoutEnvVar)

	if err := database.Initialize(); err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/", api.HandleWipeout(timeout))
	logger.Info("starting wipeout server")
	log.Fatal(http.ListenAndServe(":8080", router))
}
