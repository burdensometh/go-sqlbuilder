// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

// newBaseOrderQuery builds an assembled base order query with a CTE,
// shared filters and args. It's used to test forking partition queries.
func newBaseOrderQuery() *SelectBuilder {
	cte := With(
		CTEQuery("recent_orders").As(
			Select("id", "user_id", "status", "amount", "created_at").
				From("orders").
				Where("created_at >= '2026-01-01'"),
		),
	)
	base := PostgreSQL.NewSelectBuilder().
		With(cte).
		Select("id", "user_id", "status", "amount").
		From("recent_orders")
	base.Where(base.In("status", "paid", "shipped"))
	base.Where(base.GreaterThan("amount", 100))
	return base
}

func TestForkSelectBuilder(t *testing.T) {
	a := assert.New(t)
	base := newBaseOrderQuery()

	// Fork the assembled base query into a set of independent partition queries.
	partitions := Fork(base, 2)
	a.Equal(len(partitions), 2)

	// Reports append their own date and status conditions in parallel.
	daily := partitions[0]
	daily.Where(daily.Equal("status", "paid"))
	daily.Where(daily.Between("created_at", "2026-11-01", "2026-11-02"))

	refunds := partitions[1]
	refunds.Where(refunds.Equal("status", "refund"))
	refunds.Where(refunds.Between("created_at", "2026-10-01", "2026-11-01"))

	// The base query keeps its own filters and args scope.
	sql, args := base.Build()
	a.Equal(sql, "WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status IN ($1, $2) AND amount > $3")
	a.Equal(args, []interface{}{"paid", "shipped", 100})

	// Each partition query carries the CTE and the base filters, plus its own
	// conditions, with placeholders numbered in its own args scope.
	sql, args = daily.Build()
	a.Equal(sql, "WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status IN ($1, $2) AND amount > $3 AND status = $4 AND created_at BETWEEN $5 AND $6")
	a.Equal(args, []interface{}{"paid", "shipped", 100, "paid", "2026-11-01", "2026-11-02"})

	sql, args = refunds.Build()
	a.Equal(sql, "WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status IN ($1, $2) AND amount > $3 AND status = $4 AND created_at BETWEEN $5 AND $6")
	a.Equal(args, []interface{}{"paid", "shipped", 100, "refund", "2026-10-01", "2026-11-01"})

	// Derived queries never modify the base query or each other.
	a.Equal(base.String(), newBaseOrderQuery().String())

	dailySQL, dailyArgs := daily.Build()
	refundsSQL, refundsArgs := refunds.Build()
	a.Equal(dailySQL, refundsSQL)      // Same shape, but...
	a.NotEqual(dailyArgs, refundsArgs) // ...independent args scopes.
}

func TestForkSelectBuilderCTEIsolation(t *testing.T) {
	a := assert.New(t)

	cteb := With(
		CTEQuery("recent_orders").As(
			Select("id").From("orders").Where("created_at >= '2026-01-01'"),
		),
	)
	base := PostgreSQL.NewSelectBuilder().With(cteb).Select("id").From("recent_orders")

	partitions := Fork(base, 1)
	forked := partitions[0]

	// Rebuild the CTE after forking. The base query sees the new CTE,
	// while the forked query keeps the CTE cloned at fork time.
	cteb.With(
		CTEQuery("recent_orders").As(
			Select("id").From("orders").Where("created_at >= '2027-01-01'"),
		),
	)

	a.Equal(base.String(), "WITH recent_orders AS (SELECT id FROM orders WHERE created_at >= '2027-01-01') SELECT id FROM recent_orders")
	a.Equal(forked.String(), "WITH recent_orders AS (SELECT id FROM orders WHERE created_at >= '2026-01-01') SELECT id FROM recent_orders")
}

func TestForkUnionBuilder(t *testing.T) {
	a := assert.New(t)

	ub := PostgreSQL.NewUnionBuilder().Union(
		Select("id").From("orders_a"),
		Select("id").From("orders_b"),
	)

	partitions := Fork(ub, 2)
	a.Equal(len(partitions), 2)

	partitions[0].OrderByDesc("id")

	// The forked union is independent of the original one and its sibling.
	a.Equal(ub.String(), "(SELECT id FROM orders_a) UNION (SELECT id FROM orders_b)")
	a.Equal(partitions[0].String(), "(SELECT id FROM orders_a) UNION (SELECT id FROM orders_b) ORDER BY id DESC")
	a.Equal(partitions[1].String(), ub.String())
}

func TestForkNonPositive(t *testing.T) {
	a := assert.New(t)
	base := newBaseOrderQuery()

	a.Assert(Fork(base, 0) == nil)
	a.Assert(Fork(base, -1) == nil)
}

func ExampleFork() {
	// Assemble the base order query once, with a CTE and shared filters.
	cte := With(
		CTEQuery("recent_orders").As(
			Select("id", "user_id", "status", "amount", "created_at").
				From("orders").
				Where("created_at >= '2026-01-01'"),
		),
	)
	base := NewSelectBuilder().With(cte).
		Select("id", "user_id", "status", "amount").
		From("recent_orders")
	base.Where(base.In("status", "paid", "shipped"))

	// Fork the base query into independent partition queries, one per report.
	partitions := Fork(base, 2)

	daily := partitions[0]
	daily.Where(daily.Equal("status", "paid"))
	daily.Where(daily.Between("created_at", "2026-11-01", "2026-11-02"))

	refunds := partitions[1]
	refunds.Where(refunds.Equal("status", "refund"))
	refunds.Where(refunds.Between("created_at", "2026-10-01", "2026-11-01"))

	// The base query is not affected by the derived queries.
	fmt.Println(base.String())

	// Each partition query can be consumed by an outer INSERT ... SELECT.
	for _, q := range partitions {
		ib := NewInsertBuilder()
		ib.InsertInto("report_orders").Cols("id", "user_id", "status", "amount")
		ib.SelectBuilder(q)

		_, args := ib.Build()
		fmt.Println(args)
	}

	// Output:
	// WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') SELECT id, user_id, status, amount FROM recent_orders WHERE status IN (?, ?)
	// [paid shipped paid 2026-11-01 2026-11-02]
	// [paid shipped refund 2026-10-01 2026-11-01]
}
