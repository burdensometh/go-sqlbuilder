// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import "fmt"

// billingWindow is one side's filter set.
//
// The raw and processed sides are filtered independently: they may use a
// different status and a different time column/window. Batch keys are only
// guaranteed to exist in the union of both sides.
type billingWindow struct {
	Status string
	From   string
	To     string
}

func exampleRawAggregate(flavor Flavor, w billingWindow) Builder {
	b := flavor.NewSelectBuilder().Select(
		"batch_key",
		"COUNT(*) AS raw_count",
		"COUNT(amount_cents) AS raw_amount_rows",
		"COALESCE(SUM(amount_cents), 0) AS raw_amount_cents",
	).From("billing_raw_event")
	b.Where(
		b.E("status", w.Status),
		b.GTE("event_time", w.From),
		b.LT("event_time", w.To),
	)
	b.GroupBy("batch_key")
	return b
}

func exampleProcessedAggregate(flavor Flavor, w billingWindow) Builder {
	b := flavor.NewSelectBuilder().Select(
		"batch_key",
		"COUNT(*) AS processed_count",
		"COUNT(amount_cents) AS processed_amount_rows",
		"COALESCE(SUM(amount_cents), 0) AS processed_amount_cents",
	).From("billing_processed_record")
	b.Where(
		b.E("status", w.Status),
		b.GTE("processed_at", w.From),
		b.LT("processed_at", w.To),
	)
	b.GroupBy("batch_key")
	return b
}

// exampleBillingReconcile builds the INSERT and the diagnostic SELECT from one
// shared CTE, so both see exactly the same reconciliation rows.
func exampleBillingReconcile(flavor Flavor, raw, processed billingWindow) (insert *InsertBuilder, diagnose *SelectBuilder) {
	const (
		rawSide       = "raw_side"
		processedSide = "processed_side"
		batchKeys     = "batch_keys"
		reconciled    = "reconciled"
	)

	rawAgg := exampleRawAggregate(flavor, raw)
	processedAgg := exampleProcessedAggregate(flavor, processed)

	// Neither side is authoritative: a batch may exist in only one of them.
	keys := flavor.NewUnionBuilder().Union(
		flavor.NewSelectBuilder().Select("batch_key").From(rawSide),
		flavor.NewSelectBuilder().Select("batch_key").From(processedSide),
	)

	// Align both sides on the common batch key. COUNT() and SUM() are compared
	// separately: COALESCE here only fills in a side that is entirely absent,
	// and is_balanced additionally requires that both sides exist.
	reconciledQuery := flavor.NewSelectBuilder().Select(
		"k.batch_key AS batch_key",
		"CASE WHEN r.batch_key IS NULL THEN 0 ELSE 1 END AS has_raw",
		"CASE WHEN p.batch_key IS NULL THEN 0 ELSE 1 END AS has_processed",
		"COALESCE(r.raw_count, 0) AS raw_count",
		"COALESCE(p.processed_count, 0) AS processed_count",
		"COALESCE(r.raw_amount_rows, 0) AS raw_amount_rows",
		"COALESCE(p.processed_amount_rows, 0) AS processed_amount_rows",
		"COALESCE(r.raw_amount_cents, 0) AS raw_amount_cents",
		"COALESCE(p.processed_amount_cents, 0) AS processed_amount_cents",
		"COALESCE(r.raw_count, 0) - COALESCE(p.processed_count, 0) AS count_diff",
		"COALESCE(r.raw_amount_rows, 0) - COALESCE(p.processed_amount_rows, 0) AS amount_rows_diff",
		"COALESCE(r.raw_amount_cents, 0) - COALESCE(p.processed_amount_cents, 0) AS amount_diff_cents",
		"CASE WHEN r.batch_key IS NOT NULL AND p.batch_key IS NOT NULL"+
			" AND r.raw_count = p.processed_count"+
			" AND r.raw_amount_rows = p.processed_amount_rows"+
			" AND r.raw_amount_cents = p.processed_amount_cents"+
			" THEN 1 ELSE 0 END AS is_balanced",
	).From(batchKeys+" AS k").
		JoinWithOption(LeftJoin, rawSide+" AS r", "r.batch_key = k.batch_key").
		JoinWithOption(LeftJoin, processedSide+" AS p", "p.batch_key = k.batch_key")

	cte := flavor.NewCTEBuilder().With(
		flavor.NewCTEQueryBuilder().Table(rawSide).As(rawAgg),
		flavor.NewCTEQueryBuilder().Table(processedSide).As(processedAgg),
		flavor.NewCTEQueryBuilder().Table(batchKeys).As(keys),
		flavor.NewCTEQueryBuilder().Table(reconciled).As(reconciledQuery),
	)

	// Only balanced batches are persisted. A batch whose amount is genuinely
	// zero still qualifies as long as both sides agree on it.
	insert = flavor.NewInsertBuilder().
		InsertInto("billing_batch_summary").
		Cols("batch_key", "raw_count", "processed_count", "raw_amount_cents", "processed_amount_cents")
	insert.Select("batch_key", "raw_count", "processed_count", "raw_amount_cents", "processed_amount_cents").
		From(reconciled).
		Where("is_balanced = 1").
		With(cte)

	// Diagnostic: every batch key seen on either side, balanced or not.
	diagnose = flavor.NewSelectBuilder().Select(
		"batch_key", "has_raw", "has_processed",
		"raw_count", "processed_count",
		"raw_amount_rows", "processed_amount_rows",
		"raw_amount_cents", "processed_amount_cents",
		"count_diff", "amount_rows_diff", "amount_diff_cents", "is_balanced",
		"CASE WHEN has_raw = 0 THEN 'missing_raw'"+
			" WHEN has_processed = 0 THEN 'missing_processed'"+
			" WHEN count_diff <> 0 THEN 'count_mismatch'"+
			" WHEN amount_rows_diff <> 0 THEN 'amount_coverage_mismatch'"+
			" WHEN amount_diff_cents <> 0 THEN 'amount_mismatch'"+
			" WHEN raw_amount_cents = 0 THEN 'balanced_zero'"+
			" ELSE 'balanced' END AS diagnosis",
	).From(reconciled).
		With(cte).
		OrderBy("batch_key")

	return insert, diagnose
}

func Example_billingReconcileWrite() {
	insert, _ := exampleBillingReconcile(PostgreSQL,
		billingWindow{Status: "accepted", From: "2026-11-01", To: "2026-11-16"},
		billingWindow{Status: "settled", From: "2026-11-02", To: "2026-11-17"},
	)

	sql, args := insert.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO billing_batch_summary (batch_key, raw_count, processed_count, raw_amount_cents, processed_amount_cents) WITH raw_side AS (SELECT batch_key, COUNT(*) AS raw_count, COUNT(amount_cents) AS raw_amount_rows, COALESCE(SUM(amount_cents), 0) AS raw_amount_cents FROM billing_raw_event WHERE status = $1 AND event_time >= $2 AND event_time < $3 GROUP BY batch_key), processed_side AS (SELECT batch_key, COUNT(*) AS processed_count, COUNT(amount_cents) AS processed_amount_rows, COALESCE(SUM(amount_cents), 0) AS processed_amount_cents FROM billing_processed_record WHERE status = $4 AND processed_at >= $5 AND processed_at < $6 GROUP BY batch_key), batch_keys AS ((SELECT batch_key FROM raw_side) UNION (SELECT batch_key FROM processed_side)), reconciled AS (SELECT k.batch_key AS batch_key, CASE WHEN r.batch_key IS NULL THEN 0 ELSE 1 END AS has_raw, CASE WHEN p.batch_key IS NULL THEN 0 ELSE 1 END AS has_processed, COALESCE(r.raw_count, 0) AS raw_count, COALESCE(p.processed_count, 0) AS processed_count, COALESCE(r.raw_amount_rows, 0) AS raw_amount_rows, COALESCE(p.processed_amount_rows, 0) AS processed_amount_rows, COALESCE(r.raw_amount_cents, 0) AS raw_amount_cents, COALESCE(p.processed_amount_cents, 0) AS processed_amount_cents, COALESCE(r.raw_count, 0) - COALESCE(p.processed_count, 0) AS count_diff, COALESCE(r.raw_amount_rows, 0) - COALESCE(p.processed_amount_rows, 0) AS amount_rows_diff, COALESCE(r.raw_amount_cents, 0) - COALESCE(p.processed_amount_cents, 0) AS amount_diff_cents, CASE WHEN r.batch_key IS NOT NULL AND p.batch_key IS NOT NULL AND r.raw_count = p.processed_count AND r.raw_amount_rows = p.processed_amount_rows AND r.raw_amount_cents = p.processed_amount_cents THEN 1 ELSE 0 END AS is_balanced FROM batch_keys AS k LEFT JOIN raw_side AS r ON r.batch_key = k.batch_key LEFT JOIN processed_side AS p ON p.batch_key = k.batch_key) SELECT batch_key, raw_count, processed_count, raw_amount_cents, processed_amount_cents FROM reconciled WHERE is_balanced = 1
	// [accepted 2026-11-01 2026-11-16 settled 2026-11-02 2026-11-17]
}

func Example_billingReconcileDiagnose() {
	_, diagnose := exampleBillingReconcile(PostgreSQL,
		billingWindow{Status: "accepted", From: "2026-11-01", To: "2026-11-16"},
		billingWindow{Status: "settled", From: "2026-11-02", To: "2026-11-17"},
	)

	sql, args := diagnose.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// WITH raw_side AS (SELECT batch_key, COUNT(*) AS raw_count, COUNT(amount_cents) AS raw_amount_rows, COALESCE(SUM(amount_cents), 0) AS raw_amount_cents FROM billing_raw_event WHERE status = $1 AND event_time >= $2 AND event_time < $3 GROUP BY batch_key), processed_side AS (SELECT batch_key, COUNT(*) AS processed_count, COUNT(amount_cents) AS processed_amount_rows, COALESCE(SUM(amount_cents), 0) AS processed_amount_cents FROM billing_processed_record WHERE status = $4 AND processed_at >= $5 AND processed_at < $6 GROUP BY batch_key), batch_keys AS ((SELECT batch_key FROM raw_side) UNION (SELECT batch_key FROM processed_side)), reconciled AS (SELECT k.batch_key AS batch_key, CASE WHEN r.batch_key IS NULL THEN 0 ELSE 1 END AS has_raw, CASE WHEN p.batch_key IS NULL THEN 0 ELSE 1 END AS has_processed, COALESCE(r.raw_count, 0) AS raw_count, COALESCE(p.processed_count, 0) AS processed_count, COALESCE(r.raw_amount_rows, 0) AS raw_amount_rows, COALESCE(p.processed_amount_rows, 0) AS processed_amount_rows, COALESCE(r.raw_amount_cents, 0) AS raw_amount_cents, COALESCE(p.processed_amount_cents, 0) AS processed_amount_cents, COALESCE(r.raw_count, 0) - COALESCE(p.processed_count, 0) AS count_diff, COALESCE(r.raw_amount_rows, 0) - COALESCE(p.processed_amount_rows, 0) AS amount_rows_diff, COALESCE(r.raw_amount_cents, 0) - COALESCE(p.processed_amount_cents, 0) AS amount_diff_cents, CASE WHEN r.batch_key IS NOT NULL AND p.batch_key IS NOT NULL AND r.raw_count = p.processed_count AND r.raw_amount_rows = p.processed_amount_rows AND r.raw_amount_cents = p.processed_amount_cents THEN 1 ELSE 0 END AS is_balanced FROM batch_keys AS k LEFT JOIN raw_side AS r ON r.batch_key = k.batch_key LEFT JOIN processed_side AS p ON p.batch_key = k.batch_key) SELECT batch_key, has_raw, has_processed, raw_count, processed_count, raw_amount_rows, processed_amount_rows, raw_amount_cents, processed_amount_cents, count_diff, amount_rows_diff, amount_diff_cents, is_balanced, CASE WHEN has_raw = 0 THEN 'missing_raw' WHEN has_processed = 0 THEN 'missing_processed' WHEN count_diff <> 0 THEN 'count_mismatch' WHEN amount_rows_diff <> 0 THEN 'amount_coverage_mismatch' WHEN amount_diff_cents <> 0 THEN 'amount_mismatch' WHEN raw_amount_cents = 0 THEN 'balanced_zero' ELSE 'balanced' END AS diagnosis FROM reconciled ORDER BY batch_key
	// [accepted 2026-11-01 2026-11-16 settled 2026-11-02 2026-11-17]
}
