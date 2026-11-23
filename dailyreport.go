// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

// NewStoreDailyReportBuilder creates a SelectBuilder to query a store daily
// report. The report contains exactly one row per store per business day:
// days without any transaction are filled with zero values, and days with
// multiple transactions are pre-aggregated into a single row.
//
// The query is built in two steps:
//
//  1. daily_agg: transactions in txnTable are aggregated by store and day.
//     txnTable must have columns "store_id", "txn_at" and "amount".
//  2. store_days: the complete store-day set, i.e. the cartesian product of
//     storeIDs and all dates in bizDayTable, which must have a "biz_date"
//     column. The final SELECT left joins daily_agg to store_days and turns
//     missing rows into zeros with COALESCE.
//
// The result has columns "store_id", "biz_date", "txn_count" and
// "total_amount" and is ordered by "store_id" and "biz_date".
//
// The builder is created with the default flavor. Use SetFlavor or
// BuildWithFlavor on the returned builder to compile it for other databases.
func NewStoreDailyReportBuilder(txnTable, bizDayTable string, storeIDs ...interface{}) *SelectBuilder {
	// Step 1: aggregate transactions by store and day. Grouping first
	// guarantees at most one row per (store_id, biz_date), so the join
	// below never multiplies rows on days with many transactions.
	dailyAgg := Select(
		"store_id",
		"DATE(txn_at) AS biz_date",
		"COUNT(*) AS txn_count",
		"SUM(amount) AS total_amount",
	).
		From(txnTable).
		GroupBy("store_id", "DATE(txn_at)")
	dailyAgg.Where(dailyAgg.In("store_id", storeIDs...))

	// The caller-provided store range as a relation, so that stores without
	// any transaction still show up in the report.
	var storeSet Builder

	if len(storeIDs) == 0 {
		// An empty store range produces an empty report.
		storeSet = Select("NULL AS store_id").Where("0 = 1")
	} else {
		rows := make([]Builder, 0, len(storeIDs))

		for i, id := range storeIDs {
			row := NewSelectBuilder()

			if i == 0 {
				row.Select(row.Var(id) + " AS store_id")
			} else {
				row.Select(row.Var(id))
			}

			rows = append(rows, row)
		}

		storeSet = UnionAll(rows...)
	}

	// Step 2: the complete store-day set — every store in the range crossed
	// with every business day.
	storeDays := Select("s.store_id", "d.biz_date")
	storeDays.From(storeDays.BuilderAs(storeSet, "s")).
		JoinWithOption(JoinOption("CROSS"), bizDayTable+" AS d")

	cte := With(
		CTEQuery("store_days").As(storeDays),
		CTEQuery("daily_agg").As(dailyAgg),
	)

	return cte.Select(
		"sd.store_id",
		"sd.biz_date",
		"COALESCE(da.txn_count, 0) AS txn_count",
		"COALESCE(da.total_amount, 0) AS total_amount",
	).
		From("store_days AS sd").
		JoinWithOption(LeftJoin, "daily_agg AS da",
			"da.store_id = sd.store_id",
			"da.biz_date = sd.biz_date",
		).
		OrderBy("sd.store_id", "sd.biz_date")
}
