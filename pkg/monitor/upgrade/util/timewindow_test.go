/*
Copyright 2022-2023 EscherCloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/eschercloudai/unikorn/pkg/monitor/upgrade/util"
)

const (
	samples = 1000000
)

func logStats(t *testing.T, freqs map[int]int) {
	t.Helper()

	var mean float64

	for _, freq := range freqs {
		mean += float64(freq)
	}

	mean /= float64(len(freqs))

	var variance float64

	for _, freq := range freqs {
		diff := float64(freq) - mean

		variance += diff * diff
	}

	variance /= float64(len(freqs))

	stddev := math.Sqrt(variance)

	t.Log("mean", mean, "stddev", stddev, fmt.Sprintf("(%f%%)", stddev*100/mean))
}

// TestGenerateTimeWindow ensures time window generation spits out times when
// we expect it to.
func TestGenerateTimeWindow(t *testing.T) {
	t.Parallel()

	// Keep tabs on what days of the week and hours things get scheduled.
	daysOfWeek := map[int]int{}
	hoursOfDay := map[int]int{}

	// This is driven by RANDOM, so use enough iterations to be statistically
	// significant.
	for i := 0; i < samples; i++ {
		window := util.GenerateTimeWindow(uuid.New().String())

		// We shouldn't be triggering fails unless we're in the office to
		// deal with them.
		if window.Start.Weekday() == time.Friday || window.Start.Weekday() == time.Saturday {
			t.Fatal("start time is when we should be sipping pina coladas")
		}

		daysOfWeek[int(window.Start.Weekday())]++

		// We shouldn't be triggering fails during the working day.
		if window.Start.Hour() > 7 {
			t.Fatal("start time is during the working day")
		}

		hoursOfDay[window.Start.Hour()]++

		if window.End.Sub(window.Start) != time.Hour {
			t.Fatal("end time isn't one hour after the start time")
		}
	}

	// As a final check, ensure all the expected indices are there.
	for i := time.Sunday; i != time.Friday; i++ {
		if _, ok := daysOfWeek[int(i)]; !ok {
			t.Fatal("Nothing scheduled on", i)
		}
	}

	t.Log("day of week statistics")
	logStats(t, daysOfWeek)

	for i := 0; i < 8; i++ {
		if _, ok := hoursOfDay[i]; !ok {
			t.Fatal("Nothing scheduled at", i)
		}
	}

	t.Log("time of day statistics")
	logStats(t, hoursOfDay)
}
