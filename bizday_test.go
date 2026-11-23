// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"strings"
	"testing"
	"time"

	"github.com/huandu/go-assert"
)

func mustParseUTC(t *testing.T, s string) time.Time {
	t.Helper()

	ts, err := time.Parse(time.RFC3339, s)

	if err != nil {
		t.Fatalf("cannot parse time %q: %v", s, err)
	}

	return ts
}

func TestBusinessDayUTC(t *testing.T) {
	cases := []struct {
		date     string
		timezone string
		start    string // Expected UTC start in RFC3339.
		end      string // Expected UTC end in RFC3339.
		hours    time.Duration
	}{
		// UTC and zones without DST always have 24-hour days.
		{"2026-06-15", "UTC", "2026-06-15T00:00:00Z", "2026-06-16T00:00:00Z", 24 * time.Hour},
		{"2026-03-08", "Asia/Shanghai", "2026-03-07T16:00:00Z", "2026-03-08T16:00:00Z", 24 * time.Hour},

		// America/New_York: a regular winter day (EST, UTC-5).
		{"2026-01-15", "America/New_York", "2026-01-15T05:00:00Z", "2026-01-16T05:00:00Z", 24 * time.Hour},
		// America/New_York: DST starts on 2026-03-08. The day is 23 hours long.
		{"2026-03-08", "America/New_York", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z", 23 * time.Hour},
		// America/New_York: DST ends on 2026-11-01. The day is 25 hours long.
		{"2026-11-01", "America/New_York", "2026-11-01T04:00:00Z", "2026-11-02T05:00:00Z", 25 * time.Hour},

		// Europe/Berlin: DST starts on 2026-03-29 (23 hours) and ends on
		// 2026-10-25 (25 hours). Local midnight is ahead of UTC.
		{"2026-03-29", "Europe/Berlin", "2026-03-28T23:00:00Z", "2026-03-29T22:00:00Z", 23 * time.Hour},
		{"2026-10-25", "Europe/Berlin", "2026-10-24T22:00:00Z", "2026-10-25T23:00:00Z", 25 * time.Hour},

		// Australia/Sydney: DST is in the southern hemisphere summer. It
		// starts on 2026-10-04 (23 hours) and ends on 2026-04-05 (25 hours).
		{"2026-10-04", "Australia/Sydney", "2026-10-03T14:00:00Z", "2026-10-04T13:00:00Z", 23 * time.Hour},
		{"2026-04-05", "Australia/Sydney", "2026-04-04T13:00:00Z", "2026-04-05T14:00:00Z", 25 * time.Hour},
	}

	for _, c := range cases {
		t.Run(c.timezone+"/"+c.date, func(t *testing.T) {
			a := assert.New(t)

			start, end, err := BusinessDayUTC(c.date, c.timezone)
			a.Equal(err, nil)
			a.Equal(start, mustParseUTC(t, c.start))
			a.Equal(end, mustParseUTC(t, c.end))
			a.Equal(end.Sub(start), c.hours)

			// Both bounds must be in UTC.
			a.Equal(start.Location(), time.UTC)
			a.Equal(end.Location(), time.UTC)
		})
	}
}

func TestBusinessDayUTCError(t *testing.T) {
	a := assert.New(t)

	for _, timezone := range []string{"", "Mars/Olympus_Mons", "UTC+8"} {
		_, _, err := BusinessDayUTC("2026-03-08", timezone)
		a.Assert(err != nil)
	}

	for _, date := range []string{"", "2026-3-8", "20260308", "2026-13-01", "2026-02-30", "2026/03/08"} {
		_, _, err := BusinessDayUTC(date, "America/New_York")
		a.Assert(err != nil)
	}
}

func TestStoreDailyReportBuilder(t *testing.T) {
	a := assert.New(t)

	report := &StoreDailyReport{
		Table:      "transactions",
		Columns:    []string{"store_id", "COUNT(*) AS cnt"},
		Date:       "2026-03-08", // DST starts in America/New_York: a 23-hour day.
		TimeZone:   "America/New_York",
		MinStoreID: 100,
		MaxStoreID: 199,
	}

	sb, err := report.Builder()
	a.Equal(err, nil)

	// The builder can be further customized.
	sb.GroupBy("store_id").OrderBy("store_id").Asc()

	start := mustParseUTC(t, "2026-03-08T05:00:00Z")
	end := mustParseUTC(t, "2026-03-09T04:00:00Z")

	sql, args := sb.BuildWithFlavor(MySQL)
	a.Equal(sql, "SELECT store_id, COUNT(*) AS cnt FROM transactions WHERE created_at >= ? AND created_at < ? AND store_id BETWEEN ? AND ? GROUP BY store_id ORDER BY store_id ASC")
	a.Equal(args, []interface{}{start, end, int64(100), int64(199)})

	// The UTC bounds are interpolated as UTC wall clock.
	interpolated, err := MySQL.Interpolate(sql, args)
	a.Equal(err, nil)
	a.Assert(strings.Contains(interpolated, "created_at >= '2026-03-08 05:00:00'"))
	a.Assert(strings.Contains(interpolated, "created_at < '2026-03-09 04:00:00'"))

	sql, args = sb.BuildWithFlavor(PostgreSQL)
	a.Equal(sql, "SELECT store_id, COUNT(*) AS cnt FROM transactions WHERE created_at >= $1 AND created_at < $2 AND store_id BETWEEN $3 AND $4 GROUP BY store_id ORDER BY store_id ASC")
	a.Equal(args, []interface{}{start, end, int64(100), int64(199)})

	// Default columns and column names.
	sql, args, err = (&StoreDailyReport{
		Table:      "transactions",
		Date:       "2026-11-01", // DST ends in America/New_York: a 25-hour day.
		TimeZone:   "America/New_York",
		MinStoreID: 1,
		MaxStoreID: 50,
	}).Build()
	a.Equal(err, nil)
	a.Equal(sql, "SELECT * FROM transactions WHERE created_at >= ? AND created_at < ? AND store_id BETWEEN ? AND ?")
	a.Equal(args, []interface{}{
		mustParseUTC(t, "2026-11-01T04:00:00Z"),
		mustParseUTC(t, "2026-11-02T05:00:00Z"),
		int64(1), int64(50),
	})

	// Custom column names.
	sql, _, err = (&StoreDailyReport{
		Table:           "orders",
		StoreIDColumn:   "shop_id",
		CreatedAtColumn: "paid_at",
		Date:            "2026-01-15",
		TimeZone:        "Europe/Berlin",
		MinStoreID:      7,
		MaxStoreID:      7,
	}).Build()
	a.Equal(err, nil)
	a.Equal(sql, "SELECT * FROM orders WHERE paid_at >= ? AND paid_at < ? AND shop_id BETWEEN ? AND ?")
}

func TestStoreDailyReportBuilderError(t *testing.T) {
	a := assert.New(t)

	valid := StoreDailyReport{
		Table:      "transactions",
		Date:       "2026-03-08",
		TimeZone:   "America/New_York",
		MinStoreID: 1,
		MaxStoreID: 10,
	}

	report := valid
	report.Table = ""
	_, err := report.Builder()
	a.Assert(err != nil)

	report = valid
	report.MinStoreID, report.MaxStoreID = 10, 1
	_, err = report.Builder()
	a.Assert(err != nil)

	report = valid
	report.TimeZone = "not-a-timezone"
	_, err = report.Builder()
	a.Assert(err != nil)

	report = valid
	report.Date = "2026-02-30"
	_, err = report.Builder()
	a.Assert(err != nil)
}

// TestStoreDailyReportBoundaries verifies that a daily report never misses
// the last local transaction of the business date and never includes the
// first local transaction of the next day, including on DST transition days
// when the local day is 23 or 25 hours long.
func TestStoreDailyReportBoundaries(t *testing.T) {
	cases := []struct {
		date     string
		timezone string
	}{
		{"2026-03-08", "America/New_York"}, // Spring forward: 23 hours.
		{"2026-11-01", "America/New_York"}, // Fall back: 25 hours.
		{"2026-03-29", "Europe/Berlin"},    // Spring forward.
		{"2026-10-25", "Europe/Berlin"},    // Fall back.
		{"2026-10-04", "Australia/Sydney"}, // Spring forward.
		{"2026-04-05", "Australia/Sydney"}, // Fall back.
		{"2026-01-15", "America/New_York"}, // A regular 24-hour day.
	}

	// inReport simulates the "created_at >= start AND created_at < end"
	// semantics applied by the database.
	inReport := func(createdAt, start, end time.Time) bool {
		return !createdAt.Before(start) && createdAt.Before(end)
	}

	for _, c := range cases {
		t.Run(c.timezone+"/"+c.date, func(t *testing.T) {
			a := assert.New(t)

			loc, err := time.LoadLocation(c.timezone)
			a.Equal(err, nil)

			start, end, err := BusinessDayUTC(c.date, c.timezone)
			a.Equal(err, nil)

			// The local midnight of date and of the following day in loc.
			startLocal, err := time.ParseInLocation(businessDateFormat, c.date, loc)
			a.Equal(err, nil)
			endLocal := startLocal.AddDate(0, 0, 1)

			// The last possible local transaction of the day: one nanosecond
			// before the next local midnight. It must be in the report.
			lastOfDay := endLocal.Add(-time.Nanosecond)
			a.Assert(inReport(lastOfDay.UTC(), start, end))

			// A typical late-night transaction must be in the report as well.
			lateNight := endLocal.Add(-time.Second)
			a.Assert(inReport(lateNight.UTC(), start, end))

			// The first local transaction of the next day is exactly the next
			// local midnight. It must NOT be in the report.
			firstOfNextDay := endLocal
			a.Assert(!inReport(firstOfNextDay.UTC(), start, end))

			// The interval edges: start is included, end is excluded, and
			// anything before start is excluded.
			a.Assert(inReport(start, start, end))
			a.Assert(!inReport(end, start, end))
			a.Assert(!inReport(start.Add(-time.Nanosecond), start, end))

			// Consecutive daily partitions tile the timeline exactly: the end
			// of this day is the start of the next day, so back-to-back
			// reports neither overlap nor leave a gap.
			nextDate := endLocal.Format(businessDateFormat)
			nextStart, _, err := BusinessDayUTC(nextDate, c.timezone)
			a.Equal(err, nil)
			a.Assert(end.Equal(nextStart))

			// The generated SQL must implement the half-open interval: >= for
			// the start and < (never <=) for the end.
			sql, args, err := (&StoreDailyReport{
				Table:      "transactions",
				Date:       c.date,
				TimeZone:   c.timezone,
				MinStoreID: 1,
				MaxStoreID: 100,
			}).Build()
			a.Equal(err, nil)
			a.Assert(strings.Contains(sql, "created_at >= ?"))
			a.Assert(strings.Contains(sql, "created_at < ?"))
			a.Assert(!strings.Contains(sql, "created_at <= ?"))
			a.Equal(args[0], start)
			a.Equal(args[1], end)
		})
	}
}

// TestStoreDailyReportFallBackHour verifies that on the day DST ends, the
// repeated local hour is fully covered by the report: transactions from both
// occurrences of 01:30 local time must be included.
func TestStoreDailyReportFallBackHour(t *testing.T) {
	a := assert.New(t)

	loc, err := time.LoadLocation("America/New_York")
	a.Equal(err, nil)

	start, end, err := BusinessDayUTC("2026-11-01", "America/New_York")
	a.Equal(err, nil)

	inReport := func(createdAt time.Time) bool {
		return !createdAt.Before(start) && createdAt.Before(end)
	}

	// Go resolves the ambiguous 01:30 to the first occurrence (EDT, UTC-4).
	first := time.Date(2026, 11, 1, 1, 30, 0, 0, loc)
	a.Equal(first.UTC(), mustParseUTC(t, "2026-11-01T05:30:00Z"))

	// The second occurrence of 01:30 happens one hour later (EST, UTC-5).
	second := first.Add(time.Hour)
	a.Equal(second.UTC(), mustParseUTC(t, "2026-11-01T06:30:00Z"))
	a.Equal(second.In(loc).Format("15:04"), "01:30")

	a.Assert(inReport(first.UTC()))
	a.Assert(inReport(second.UTC()))
}
