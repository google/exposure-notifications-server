// +build performance

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

package performance

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	// Warning: package github.com/golang/protobuf/proto is deprecated: Use the
	// "google.golang.org/protobuf/proto" package instead.  (SA1019).
	// Use deprecated package as mako isn't compatible with
	// github.com/golang/protobuf/proto yet
	"github.com/golang/protobuf/proto"
	"github.com/google/mako/go/quickstore"
	"github.com/sethvargo/go-envconfig"

	qpb "github.com/google/mako/proto/quickstore/quickstore_go_proto"
	mpb "github.com/google/mako/spec/proto/mako_go_proto"
)

const (
	benchmarkConfigFile = "export_test_benchmark.config"
)

type config struct {
	MakoPort uint `env:"MAKO_PORT,required"`
}

// setup sets up client used for performance test
func setup(tb testing.TB) (*quickstore.Quickstore, func(context.Context)) {
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	benchmarkConfig := &mpb.BenchmarkInfo{}
	data, err := ioutil.ReadFile(benchmarkConfigFile)
	if err != nil {
		tb.Fatal(err)
	}
	if err = proto.UnmarshalText(string(data), benchmarkConfig); err != nil {
		tb.Fatal(err)
	}
	input := &qpb.QuickstoreInput{
		BenchmarkKey: proto.String(*benchmarkConfig.BenchmarkKey),
	}

	c := config{}
	if err := envconfig.ProcessWith(context.Background(), &c, envconfig.OsLookuper()); err != nil {
		tb.Fatalf("unable to process env: %v", err)
	}
	microservice := fmt.Sprintf("localhost:%d", c.MakoPort)

	q, close, err := quickstore.NewAtAddress(ctxWithTimeout, input, microservice)
	if err != nil {
		tb.Fatalf("quickstore.NewAtAddress() = %v", err)
	}
	return q, close
}

func store(tb testing.TB, q *quickstore.Quickstore) {
	out, err := q.Store()
	if err != nil {
		tb.Fatalf("quickstore Store() = %s", err)
	}
	switch out.GetStatus() {
	case qpb.QuickstoreOutput_SUCCESS:
		tb.Logf("Done! Run can be found at: %s\n", out.GetRunChartLink())
	case qpb.QuickstoreOutput_ERROR:
		tb.Fatalf("quickstore Store() output error: %s\n", out.GetSummaryOutput())
	case qpb.QuickstoreOutput_ANALYSIS_FAIL:
		tb.Fatalf("Quickstore analysis failure: %s\nRun can be found at: %s\n", out.GetSummaryOutput(), out.GetRunChartLink())
	}
}
