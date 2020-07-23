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
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/mako/go/quickstore"

	qpb "github.com/google/mako/proto/quickstore/quickstore_go_proto"
	mpb "github.com/google/mako/spec/proto/mako_go_proto"
)

const (
	benchmarkConfigFile = "export_test_benchmark.config"
)

// setup sets up client used for performance test
func setup(tb testing.TB) (*quickstore.Quickstore, func(context.Context)) {
	var (
		microservice string
	)

	benchmarkConfig := &mpb.BenchmarkInfo{}

	{
		p := os.Getenv("MAKO_PORT")
		if p == "" {
			tb.Fatal("Env var MAKO_PORT is required.")
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			tb.Fatalf("Could not parse MAKO_PORT env var: %v", err)
		}
		microservice = fmt.Sprintf("localhost:%d", port)
	}

	{
		data, err := ioutil.ReadFile(benchmarkConfigFile)
		if err != nil {
			tb.Fatal(err)
		}
		if err = proto.UnmarshalText(string(data), benchmarkConfig); err != nil {
			tb.Fatal(err)
		}
	}

	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	input := &qpb.QuickstoreInput{
		BenchmarkKey: proto.String(*benchmarkConfig.BenchmarkKey),
	}

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
