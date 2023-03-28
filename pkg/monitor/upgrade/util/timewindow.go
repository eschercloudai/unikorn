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

package util

import (
	"crypto/sha256"
	"time"
)

type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// GenerateTimeWindow returns a one hour time window in which to trigger a
// resource upgrade.  It's based on hashing to get a fairly uniform
// distribution over time, so as to avoid everything getting done at
// once... and the consequences when stuff goes wrong!
func GenerateTimeWindow(name string) *TimeWindow {
	sum := sha256.Sum256([]byte(name))

	// So, counter to what's intuative, allow this to run
	// Sunday-Thursday, so we're in normal working hours to
	// fix any problems.
	dayOfTheWeek := int(sum[0]) % 5

	// Then stagger upgrades between 00:00 and 7:00 UTC, that
	// gets most things out of the way before 8:00 CET (+01:00),
	// or 08:00/09:00 respectively when BST/CEST kicks in.
	hourOfTheDay := int(sum[1]) % 8

	now := time.Now()

	// Now here's where things get kinda complex... start by rounding down
	// the current time to the previous/current Sunday, then add on our selected day.
	// Golang's weekdays are indexed from zero, and if it ends up negative, the library
	// will do the right thing.
	day := now.Day() - int(now.Weekday()) + dayOfTheWeek

	// Problematically, as with all things time related, it's not constant, in the
	// sense that days may miss an hour, or have the same hour twice.  But, what the
	// hell it'll upgrade next week, right?
	start := time.Date(now.Year(), now.Month(), day, hourOfTheDay, 0, 0, 0, time.UTC)

	return &TimeWindow{Start: start, End: start.Add(time.Hour)}
}

// In tells us if we are in the time window.
func (t *TimeWindow) In() bool {
	now := time.Now()

	return now.After(t.Start) && now.Before(t.End)
}
