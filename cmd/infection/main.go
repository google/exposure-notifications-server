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

// This package is the primary infected keys upload service.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/googlepartners/exposure-notifications/internal/api"
	"github.com/googlepartners/exposure-notifications/internal/api/config"
	"github.com/googlepartners/exposure-notifications/internal/database"
	"github.com/googlepartners/exposure-notifications/internal/logging"
	"github.com/googlepartners/exposure-notifications/internal/serverenv"

	"contrib.go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// TODO(beggers) delete this example metric once we have useful ones.
	dbOpenTimeMs      = stats.Int64("db/open", "Latency in ms to open a db connection", "ms")
	dbOpenLatencyView = &view.View{
		Name:        "db/open",
		Measure:     dbOpenTimeMs,
		Description: "Distribution of time to open a DB connection",
		Aggregation: view.Distribution(0, 5, 10, 15, 20, 30, 40, 50, 100, 150, 200, 250, 400, 800, 1600),
	}
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		log.Fatalf("Failed to register server views for HTTP metrics: %v", err)
	}
	if err := view.Register(dbOpenLatencyView); err != nil {
		log.Fatalf("Failed to register db open latency ms view: %v", err)
	}
	//TODO(beggers): We need to export to Stackdriver too. And have a flag
	// to choose which one to export to.
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "demo",
	})
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	dbOpenStart := time.Now()
	db, err := database.NewFromEnv(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	stats.Record(ctx, dbOpenTimeMs.M(time.Now().Sub(dbOpenStart).Milliseconds()))
	defer db.Close(ctx)

	cfg := config.New(db)
	env := serverenv.New(ctx)

	http.Handle("/metrics", pe)
	http.Handle("/v1", &ochttp.Handler{Handler: api.NewPublishHandler(db, cfg)})
	logger.Info("starting infection server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port()), nil))
}
