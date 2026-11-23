// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

// invoiceSource builds the landing-table query used by the governance rule.
// Several sources may land the same invoice in one day; the rule keeps the row
// with the highest updated_at, breaking ties by source priority.
func invoiceSource() *SelectBuilder {
	sb := NewSelectBuilder()
	sb.Select("id", "invoice_no", "source_code", "updated_at", "amount", "payload")
	sb.From("stg_invoice")
	sb.Where(sb.Equal("load_date", "2026-11-23"))
	return sb
}

func ExampleRowNumberBuilder_invoiceGovernance() {
	rnb := NewRowNumberBuilder()
	rnb.From(invoiceSource())

	// Composite business key: an invoice is identified by its number.
	// NULLIF makes NULL and an empty string the same empty key.
	rnb.PartitionBy(NullIfEmpty("invoice_no"))

	// Explicit total order: latest update wins, then the higher-priority source.
	rnb.OrderByDesc("updated_at").OrderByDesc("source_priority")

	// Stable result even when every sort value is equal.
	rnb.TieBreaker("id")

	sql, args := rnb.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, source_priority DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1
	// [2026-11-23]
}

func ExampleRowNumberBuilder_insertSelect() {
	rnb := PostgreSQL.NewRowNumberBuilder()
	rnb.From(invoiceSource())
	rnb.PartitionBy(NullIfEmpty("invoice_no"))
	rnb.OrderByDesc("updated_at").OrderByDesc("source_priority").TieBreaker("id")

	// Only business columns are projected, in target-table order.
	rnb.Select("invoice_no", "source_code", "updated_at", "amount", "payload")

	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("dwd_invoice")
	ib.Cols("invoice_no", "source_code", "updated_at", "amount", "payload")
	insertSelect := ib.Select("*")
	insertSelect.From(insertSelect.BuilderAs(rnb, "dedup"))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO dwd_invoice (invoice_no, source_code, updated_at, amount, payload) SELECT * FROM (SELECT invoice_no, source_code, updated_at, amount, payload FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, source_priority DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = $1) AS t) AS t WHERE rn = 1) AS dedup
	// [2026-11-23]
}

func ExampleNullIfEmpty() {
	sb := NewSelectBuilder()
	rnb := NewRowNumberBuilder()
	rnb.From(sb.Select("id", "invoice_no").From("stg_invoice"))
	rnb.PartitionBy(NullIfEmpty("invoice_no"))
	rnb.OrderByDesc("updated_at").TieBreaker("id")

	fmt.Println(rnb)

	// Output:
	// SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no FROM stg_invoice) AS t) AS t WHERE rn = 1
}

func ExampleCoalesceEmpty() {
	sb := NewSelectBuilder()
	rnb := NewRowNumberBuilder()
	rnb.From(sb.Select("id", "invoice_no").From("stg_invoice"))
	rnb.PartitionBy(CoalesceEmpty("invoice_no"))
	rnb.OrderByAsc(CoalesceEmpty("invoice_no")).TieBreaker("id")

	fmt.Println(rnb)

	// Output:
	// SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY COALESCE(invoice_no, '') ORDER BY COALESCE(invoice_no, '') ASC, id ASC) AS rn, * FROM (SELECT id, invoice_no FROM stg_invoice) AS t) AS t WHERE rn = 1
}

func TestRowNumberBuilderBuild(t *testing.T) {
	a := assert.New(t)

	rnb := newRowNumberBuilder()
	rnb.From(invoiceSource())
	rnb.PartitionBy("tenant_id", NullIfEmpty("invoice_no"))
	rnb.OrderByDesc("updated_at").OrderByDesc("source_priority")
	rnb.TieBreaker("id")

	sql, args := rnb.Build()
	a.Equal(sql, "SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY tenant_id, NULLIF(invoice_no, '') ORDER BY updated_at DESC, source_priority DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1")
	a.Equal(args, []interface{}{"2026-11-23"})
}

func TestRowNumberBuilderCustomColumnsAndAlias(t *testing.T) {
	a := assert.New(t)

	rnb := newRowNumberBuilder()
	rnb.From(invoiceSource())
	rnb.PartitionBy(NullIfEmpty("invoice_no"))
	rnb.OrderByDesc("updated_at").TieBreaker("id")
	rnb.RowNumberCol("row_num").DerivedAlias("src")
	rnb.Select("invoice_no", "amount")
	rnb.OrderResultBy("invoice_no ASC")
	rnb.Limit(100)

	sql, args := rnb.BuildWithFlavor(PostgreSQL)
	a.Equal(sql, "SELECT invoice_no, amount FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS row_num, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = $1) AS src) AS src WHERE row_num = 1 ORDER BY invoice_no ASC LIMIT $2")
	a.Equal(args, []interface{}{"2026-11-23", 100})
}

func TestRowNumberBuilderNoPartition(t *testing.T) {
	a := assert.New(t)

	// Without PARTITION BY, all rows form one partition and only the global first row is kept.
	rnb := newRowNumberBuilder()
	source := Select("id", "updated_at").From("stg_invoice")
	rnb.From(source).OrderByDesc("updated_at").TieBreaker("id")

	sql, _ := rnb.Build()
	a.Equal(sql, "SELECT * FROM (SELECT ROW_NUMBER() OVER (ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, updated_at FROM stg_invoice) AS t) AS t WHERE rn = 1")
}

func TestRowNumberBuilderFlavor(t *testing.T) {
	a := assert.New(t)

	rnb := ClickHouse.NewRowNumberBuilder()
	a.Equal(rnb.Flavor(), ClickHouse)

	old := rnb.SetFlavor(MySQL)
	a.Equal(old, ClickHouse)
	a.Equal(rnb.Flavor(), MySQL)

	rnb.From(invoiceSource()).PartitionBy(NullIfEmpty("invoice_no")).
		OrderByDesc("updated_at").TieBreaker("id").
		Limit(10).Offset(5)

	sql, args := rnb.Build()
	a.Equal(sql, "SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1 LIMIT ? OFFSET ?")
	a.Equal(args, []interface{}{"2026-11-23", 10, 5})
}

func TestRowNumberBuilderSQLServerLimitOffset(t *testing.T) {
	a := assert.New(t)

	rnb := SQLServer.NewRowNumberBuilder()
	rnb.From(invoiceSource()).PartitionBy(NullIfEmpty("invoice_no")).
		OrderByDesc("updated_at").TieBreaker("id").
		Limit(10).Offset(5)

	sql, args := rnb.Build()
	a.Equal(sql, "SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = @p1) AS t) AS t WHERE rn = 1 OFFSET @p2 ROWS FETCH NEXT @p3 ROWS ONLY")
	a.Equal(args, []interface{}{"2026-11-23", 5, 10})
}

func TestRowNumberBuilderNestedInSelect(t *testing.T) {
	a := assert.New(t)

	outer := NewSelectBuilder()
	rnb := newRowNumberBuilder()
	rnb.From(invoiceSource()).PartitionBy(NullIfEmpty("invoice_no")).
		OrderByDesc("updated_at").TieBreaker("id")

	outer.Select("invoice_no", "amount")
	outer.From(outer.BuilderAs(rnb, "dedup"))
	outer.Where(outer.GreaterThan("amount", 0))

	sql, args := outer.Build()
	a.Equal(sql, "SELECT invoice_no, amount FROM (SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1) AS dedup WHERE amount > ?")
	a.Equal(args, []interface{}{"2026-11-23", 0})
}

func TestRowNumberBuilderClone(t *testing.T) {
	a := assert.New(t)

	base := NewRowNumberBuilder()
	base.From(invoiceSource())
	base.PartitionBy(NullIfEmpty("invoice_no"))
	base.OrderByDesc("updated_at").TieBreaker("id")

	first := base.Clone()
	first.Select("invoice_no", "amount")

	second := base.Clone()
	second.OrderResultBy("invoice_no ASC")

	// The clone keeps the full projection; first's Select must not leak into it.
	firstSQL, _ := first.Build()
	secondSQL, _ := second.Build()
	a.Equal(firstSQL, "SELECT invoice_no, amount FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1")
	a.Equal(secondSQL, "SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1 ORDER BY invoice_no ASC")
}

func TestRowNumberBuilderSQLInjection(t *testing.T) {
	a := assert.New(t)

	rnb := newRowNumberBuilder()
	rnb.From(invoiceSource()).PartitionBy(NullIfEmpty("invoice_no")).
		OrderByDesc("updated_at").TieBreaker("id")
	rnb.SQL("/* governance dedup */")

	sql, _ := rnb.Build()
	a.Equal(sql, "SELECT * FROM (SELECT ROW_NUMBER() OVER (PARTITION BY NULLIF(invoice_no, '') ORDER BY updated_at DESC, id ASC) AS rn, * FROM (SELECT id, invoice_no, source_code, updated_at, amount, payload FROM stg_invoice WHERE load_date = ?) AS t) AS t WHERE rn = 1 /* governance dedup */")
}

func TestRowNumberBuilderMissingSourcePanics(t *testing.T) {
	a := assert.New(t)

	defer func() {
		a.Assert(recover() != nil)
	}()

	rnb := newRowNumberBuilder()
	rnb.OrderByDesc("updated_at")
	rnb.Build()
}

func TestRowNumberBuilderMissingOrderByPanics(t *testing.T) {
	a := assert.New(t)

	defer func() {
		a.Assert(recover() != nil)
	}()

	rnb := newRowNumberBuilder()
	rnb.From(invoiceSource())
	rnb.PartitionBy(NullIfEmpty("invoice_no"))
	rnb.Build()
}
