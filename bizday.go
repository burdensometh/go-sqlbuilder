// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"time"
)

// businessDateFormat is the only accepted format of a business date.
const businessDateFormat = "2006-01-02"

// BusinessDayUTC converts a local business date observed in timezone to the
// UTC half-open interval [start, end) covering exactly that local day.
//
// date must be in "yyyy-mm-dd" format and timezone must be an IANA time zone
// name, e.g. "America/New_York". The time zone must be given explicitly:
// stores in different zones observe different local days, and the local day
// is never derived from the server's local zone. An empty timezone is
// rejected instead of silently falling back to UTC.
//
// start is the local midnight of date and end is the local midnight of the
// following day, both converted to UTC. The interval is 23 or 25 hours long
// on days when daylight saving time starts or ends in timezone and 24 hours
// otherwise, so it always covers the whole local day exactly once.
//
// The interval is half-open: start <= t < end. It includes the very last
// instant of the local day and excludes the first instant of the next day.
// As the end of one day is exactly the start of the next, consecutive daily
// reports built with these bounds never overlap and never leave a gap.
func BusinessDayUTC(date, timezone string) (start, end time.Time, err error) {
	if timezone == "" {
		// time.LoadLocation would silently return UTC for an empty name,
		// which can make a store report on the wrong local day. Reject it.
		return time.Time{}, time.Time{}, fmt.Errorf("sqlbuilder: time zone is empty")
	}

	loc, err := time.LoadLocation(timezone)

	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("sqlbuilder: invalid time zone %q: %w", timezone, err)
	}

	startLocal, err := time.ParseInLocation(businessDateFormat, date, loc)

	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("sqlbuilder: invalid business date %q: %w", date, err)
	}

	// AddDate rolls over to the next local midnight in loc and honors DST
	// transitions. Never use Add(24*time.Hour) here: a local day is not
	// guaranteed to be 24 hours long.
	endLocal := startLocal.AddDate(0, 0, 1)

	return startLocal.UTC(), endLocal.UTC(), nil
}

// StoreDailyReport describes a daily partition report over the transactions
// of a range of stores on one local business day. Use Builder to build the
// query.
type StoreDailyReport struct {
	// Table is the name of the table storing transactions. It is required.
	Table string

	// Columns are the columns to select. If it is empty, "*" is selected.
	Columns []string

	// StoreIDColumn is the name of the column storing store IDs.
	// If it is empty, "store_id" is used.
	StoreIDColumn string

	// CreatedAtColumn is the name of the column storing the UTC time at
	// which a transaction was created. If it is empty, "created_at" is used.
	CreatedAtColumn string

	// Date is the local business date in "yyyy-mm-dd" format, e.g. "2026-03-08".
	Date string

	// TimeZone is the IANA time zone name in which Date is observed,
	// e.g. "America/New_York".
	TimeZone string

	// MinStoreID and MaxStoreID are the inclusive bounds of the store ID
	// range covered by this report partition.
	MinStoreID int64
	MaxStoreID int64
}

// Builder builds the report query and returns it as a SelectBuilder, so that
// callers can further customize it, e.g. adding GROUP BY or ORDER BY clauses.
//
// The UTC bounds of the local business day are computed in Go with
// BusinessDayUTC and passed to the query as ordinary arguments through the
// existing GE/LT/Between filters, so the database needs no time zone support:
//
//	created_at >= startUTC AND created_at < endUTC
//	AND store_id BETWEEN minStoreID AND maxStoreID
//
// The half-open interval guarantees that the report includes the last local
// transaction of Date and excludes the first local transaction of the next
// day, even on DST transition days.
func (r *StoreDailyReport) Builder() (*SelectBuilder, error) {
	if r.Table == "" {
		return nil, fmt.Errorf("sqlbuilder: StoreDailyReport.Table is empty")
	}

	if r.MinStoreID > r.MaxStoreID {
		return nil, fmt.Errorf("sqlbuilder: StoreDailyReport store range [%d, %d] is invalid", r.MinStoreID, r.MaxStoreID)
	}

	start, end, err := BusinessDayUTC(r.Date, r.TimeZone)

	if err != nil {
		return nil, err
	}

	storeIDColumn := r.StoreIDColumn

	if storeIDColumn == "" {
		storeIDColumn = "store_id"
	}

	createdAtColumn := r.CreatedAtColumn

	if createdAtColumn == "" {
		createdAtColumn = "created_at"
	}

	columns := r.Columns

	if len(columns) == 0 {
		columns = []string{"*"}
	}

	sb := NewSelectBuilder()
	sb.Select(columns...).From(r.Table).Where(
		sb.GE(createdAtColumn, start),
		sb.LT(createdAtColumn, end),
		sb.Between(storeIDColumn, r.MinStoreID, r.MaxStoreID),
	)
	return sb, nil
}

// Build builds the report query and returns the SQL and the args. It's a
// shortcut for calling Builder and then SelectBuilder.Build.
func (r *StoreDailyReport) Build() (sql string, args []interface{}, err error) {
	sb, err := r.Builder()

	if err != nil {
		return "", nil, err
	}

	sql, args = sb.Build()
	return sql, args, nil
}
