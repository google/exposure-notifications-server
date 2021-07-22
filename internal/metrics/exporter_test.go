// Copyright 2020 the Exposure Notifications Server authors
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

package metrics

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		f    func(e Exporter)
		want string
	}{
		{
			name: "WriteBool",
			f:    func(e Exporter) { e.WriteBool("test/bool", true) },
			want: "!METRIC! Type = test/bool cumulative = false value = true",
		},
		{
			name: "WriteInt",
			f:    func(e Exporter) { e.WriteInt("test/int", true, 6) },
			want: "!METRIC! Type = test/int cumulative = true value = 6",
		},
		{
			name: "WriteInt64",
			f:    func(e Exporter) { e.WriteInt64("test/int64", false, int64(42)) },
			want: "!METRIC! Type = test/int64 cumulative = false value = 42",
		},
		{
			name: "WriteIntDistribution",
			f: func(e Exporter) {
				e.WriteIntDistribution("test/int/distribution", true, []int{2, 4, 6, 8, 10})
			},
			want: "!METRIC! Type = test/int/distribution cumulative = true value = [2 4 6 8 10]",
		},
		{
			name: "WriteFloat64",
			f:    func(e Exporter) { e.WriteFloat64("test/float64", false, 3.14) },
			want: "!METRIC! Type = test/float64 cumulative = false value = 3.14",
		},
		{
			name: "WriteFloat64Distribution",
			f:    func(e Exporter) { e.WriteFloat64Distribution("test/float64d", true, []float64{3.14, 6.28}) },
			want: "!METRIC! Type = test/float64d cumulative = true value = [3.14 6.28]",
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			hook := func(e zapcore.Entry) error {
				got = append(got, e.Message)
				return nil
			}
			logger := zap.NewExample(zap.Hooks(hook)).Sugar()
			metrics := NewLogsBasedExporter(logger)

			c.f(metrics)

			want := []string{c.want}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
			}
		})
	}
}
