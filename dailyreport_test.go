// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleNewStoreDailyReportBuilder() {
	sb := NewStoreDailyReportBuilder("transactions", "business_days", 101, 102)
	sql, args := sb.Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// WITH store_days AS (SELECT s.store_id, d.biz_date FROM ((SELECT ? AS store_id) UNION ALL (SELECT ?)) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE store_id IN (?, ?) GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date
	// [101 102 101 102]
}

func TestNewStoreDailyReportBuilder(t *testing.T) {
	assert := assert.New(t)

	// The store range is inlined twice: once to build the complete
	// store-day set and once to scope the daily aggregation.
	sb := NewStoreDailyReportBuilder("transactions", "business_days", 101, 102)
	sql, args := sb.Build()
	assert.Equal(sql, "WITH store_days AS (SELECT s.store_id, d.biz_date FROM ((SELECT ? AS store_id) UNION ALL (SELECT ?)) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE store_id IN (?, ?) GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date")
	assert.Equal(args, []interface{}{101, 102, 101, 102})

	// PostgreSQL placeholders.
	sql, args = sb.BuildWithFlavor(PostgreSQL)
	assert.Equal(sql, "WITH store_days AS (SELECT s.store_id, d.biz_date FROM ((SELECT $1 AS store_id) UNION ALL (SELECT $2)) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE store_id IN ($3, $4) GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date")
	assert.Equal(args, []interface{}{101, 102, 101, 102})

	// SQLite doesn't parenthesize UNION branches.
	sql, args = sb.BuildWithFlavor(SQLite)
	assert.Equal(sql, "WITH store_days AS (SELECT s.store_id, d.biz_date FROM (SELECT ? AS store_id UNION ALL SELECT ?) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE store_id IN (?, ?) GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date")
	assert.Equal(args, []interface{}{101, 102, 101, 102})
}

func TestNewStoreDailyReportBuilderStoreRange(t *testing.T) {
	assert := assert.New(t)

	// A single store produces a one-row store set without UNION ALL.
	sb := NewStoreDailyReportBuilder("transactions", "business_days", 7)
	sql, args := sb.Build()
	assert.Equal(sql, "WITH store_days AS (SELECT s.store_id, d.biz_date FROM ((SELECT ? AS store_id)) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE store_id IN (?) GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date")
	assert.Equal(args, []interface{}{7, 7})

	// An empty store range yields an empty report without broken SQL.
	sb = NewStoreDailyReportBuilder("transactions", "business_days")
	sql, args = sb.Build()
	assert.Equal(sql, "WITH store_days AS (SELECT s.store_id, d.biz_date FROM (SELECT NULL AS store_id WHERE 0 = 1) AS s CROSS JOIN business_days AS d), daily_agg AS (SELECT store_id, DATE(txn_at) AS biz_date, COUNT(*) AS txn_count, SUM(amount) AS total_amount FROM transactions WHERE 0 = 1 GROUP BY store_id, DATE(txn_at)) SELECT sd.store_id, sd.biz_date, COALESCE(da.txn_count, 0) AS txn_count, COALESCE(da.total_amount, 0) AS total_amount FROM store_days AS sd LEFT JOIN daily_agg AS da ON da.store_id = sd.store_id AND da.biz_date = sd.biz_date ORDER BY sd.store_id, sd.biz_date")
	assert.Equal(args, []interface{}(nil))
}
