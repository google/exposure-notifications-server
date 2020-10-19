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

// This utility takes an exposure notifications interval number and turns
// it into a timestamp.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

func main() {
	intervalFlag := flag.Int("interval", 0, "interval to covert to timestamp")
	showChart := flag.Bool("chart", true, "show an interval chart for +/- 20 days")
	flag.Parse()

	if interval := *intervalFlag; interval != 0 {
		intTime := model.TimeForIntervalNumber(int32(interval))
		fmt.Printf("interval %v is at: %v\n", interval, intTime)
	}

	if *showChart {
		now := time.Now().UTC()
		fmt.Println("Current interval information:")
		fmt.Printf("     Current Time: %v\n", now)
		fmt.Printf(" Current Interval: %v\n", model.IntervalNumber(now))

		// Truncate to beginning of day.
		now = timeutils.Midnight(now)
		fmt.Println("Interval Chart")
		for d := -20; d <= 20; d++ {
			adj := now.Add(time.Duration(d) * (24 * time.Hour))
			fmt.Printf("+/- %3d days, interval: %7d UTC day: %v\n", d, model.IntervalNumber(adj), adj)
		}
	}
}
