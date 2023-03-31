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
	"context"
	"crypto/sha256"
	"time"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UpgradeableResource is a resource type that allows upgrades.
type UpgradeableResource interface {
	// Entropy returns a unique and random source of entropy from
	// the resources.
	Entropy() []byte

	// UpgradeSpec is the upgrade specification.
	UpgradeSpec() *unikornv1.ApplicationBundleAutoUpgradeSpec
}

type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// In tells us if we are in the time window.
func (t *TimeWindow) In() bool {
	if t == nil {
		return false
	}

	now := time.Now()

	return now.After(t.Start) && now.Before(t.End)
}

// GenerateTimeWindow returns a one hour time window in which to trigger a
// resource upgrade.  It's based on hashing to get a fairly uniform
// distribution over time, so as to avoid everything getting done at
// once... and the consequences when stuff goes wrong!
func autoTimeWindow(ctx context.Context, r UpgradeableResource) *TimeWindow {
	log := log.FromContext(ctx)

	log.Info("auto generating time window")

	sum := sha256.Sum256(r.Entropy())

	// So, counter to what's intuative (at the weekend), allow this to
	// run Monday-Friday, so we're in normal working hours to
	// fix any problems.  Go's Weekday type is zero indexed starting on
	// Sunday.
	dayOfTheWeek := (int(sum[0]) % 5) + 1

	// Then stagger upgrades between 00:00 and 7:00 UTC, that
	// gets most things out of the way before 8:00 CET (+01:00),
	// or 08:00/09:00 respectively when BST/CEST kicks in.
	hourOfTheDay := int(sum[1]) % 7

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

func timeWindowFromWeekDayWindow(ctx context.Context, r UpgradeableResource, basis time.Time, window *unikornv1.ApplicationBundleAutoUpgradeWindowSpec) *TimeWindow {
	if window == nil {
		return nil
	}

	log := log.FromContext(ctx)

	windowStart := time.Date(basis.Year(), basis.Month(), basis.Day(), window.Start, 0, 0, 0, time.UTC)

	// If the end time is before the start time, we're wrapping into the next day.
	// If it's equal, assume it covers the full 24 hour period, and not nothing.
	endDay := basis.Day()
	if window.End <= window.Start {
		endDay++
	}

	windowEnd := time.Date(basis.Year(), basis.Month(), endDay, window.End, 0, 0, 0, time.UTC)

	log.Info("considering window", "start", windowStart, "end", windowEnd)

	// If we're not in the window, crack on.  If we are however, select a hour window
	// based on entropy for load balancing purposes.
	tw := &TimeWindow{Start: windowStart, End: windowEnd}
	if !tw.In() {
		log.Info("ouside window, ignoring")

		return nil
	}

	// Determine the window size in hours...
	windowLength := window.End - window.Start
	if windowLength <= 0 {
		windowLength += 24
	}

	// ... select a deterministic hour within that window...
	sum := sha256.Sum256(r.Entropy())

	windowHour := int(sum[0]) % windowLength

	// Then finally return an hour window to run the upgrade in.
	start := windowStart.Add(time.Hour * time.Duration(windowHour))

	return &TimeWindow{Start: start, End: start.Add(time.Hour)}
}

func weekDayTimeWindow(ctx context.Context, r UpgradeableResource, weekday *unikornv1.ApplicationBundleAutoUpgradeWeekDaySpec) *TimeWindow {
	log := log.FromContext(ctx)

	log.Info("using day of the week time window")

	now := time.Now()

	// prev is the previous day's window, as that can overflow into today
	// e.g. from 22:00-07:00.
	var prev *unikornv1.ApplicationBundleAutoUpgradeWindowSpec

	// curr is the current day's window.
	var curr *unikornv1.ApplicationBundleAutoUpgradeWindowSpec

	switch now.Weekday() {
	case time.Sunday:
		prev = weekday.Saturday
		curr = weekday.Sunday
	case time.Monday:
		prev = weekday.Sunday
		curr = weekday.Monday
	case time.Tuesday:
		prev = weekday.Monday
		curr = weekday.Tuesday
	case time.Wednesday:
		prev = weekday.Tuesday
		curr = weekday.Wednesday
	case time.Thursday:
		prev = weekday.Wednesday
		curr = weekday.Thursday
	case time.Friday:
		prev = weekday.Thursday
		curr = weekday.Friday
	case time.Saturday:
		prev = weekday.Friday
		curr = weekday.Saturday
	}

	// Check yesterday's window, as it may still be active...
	if prev != nil {
		yesterday := now.Add(-time.Hour * 24)

		if window := timeWindowFromWeekDayWindow(ctx, r, yesterday, prev); window != nil {
			return window
		}
	}

	return timeWindowFromWeekDayWindow(ctx, r, now, curr)
}

func TimeWindowFromResource(ctx context.Context, r UpgradeableResource) *TimeWindow {
	spec := r.UpgradeSpec()

	switch {
	case spec.WeekDay != nil:
		return weekDayTimeWindow(ctx, r, spec.WeekDay)
	default:
		return autoTimeWindow(ctx, r)
	}
}
