// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleInsertInto() {
	sql, args := InsertInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleInsertIgnoreInto() {
	sql, args := InsertIgnoreInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT IGNORE INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleReplaceInto() {
	sql, args := ReplaceInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// REPLACE INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleInsertBuilder() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_flavorOracle() {
	ib := Oracle.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name", "status")
	ib.Values(1, "Huan Du", 1)
	ib.Values(2, "Charmy Liu", 1)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT ALL INTO demo.user (id, name, status) VALUES (:1, :2, :3) INTO demo.user (id, name, status) VALUES (:4, :5, :6) SELECT 1 from DUAL
	// [1 Huan Du 1 2 Charmy Liu 1]
}

func ExampleInsertBuilder_insertIgnore() {
	ib := NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT IGNORE INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_postgres() {
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at) VALUES ($1, $2, $3, UNIX_TIMESTAMP(NOW())), ($4, $5, $6, $7) ON CONFLICT DO NOTHING
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_sqlite() {
	ib := SQLite.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT OR IGNORE INTO demo.user (id, name, status, created_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_clickhouse() {
	ib := ClickHouse.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_replaceInto() {
	ib := NewInsertBuilder()
	ib.ReplaceInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// REPLACE INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_SQL() {
	ib := NewInsertBuilder()
	ib.SQL("/* before */")
	ib.InsertInto("demo.user")
	ib.SQL("PARTITION (p0)")
	ib.Cols("id", "name", "status", "created_at")
	ib.SQL("/* after cols */")
	ib.Values(3, "Shawn Du", 1, 1234567890)
	ib.SQL(ib.Var(Build("ON DUPLICATE KEY UPDATE status = $?", 1)))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// /* before */ INSERT INTO demo.user PARTITION (p0) (id, name, status, created_at) /* after cols */ VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE status = ?
	// [3 Shawn Du 1 1234567890 1]
}

func ExampleInsertBuilder_subSelect() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = ?
	// [1]
}

func ExampleInsertBuilder_subSelect_oracle() {
	ib := Oracle.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = :1
	// [1]
}

func ExampleInsertBuilder_subSelect_informix() {
	ib := Informix.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = ?
	// [1]
}

func ExampleInsertBuilder_SelectBuilder() {
	// A query built elsewhere, e.g. a partition query forked from a base query.
	sb := NewSelectBuilder()
	sb.Select("id", "name").From("demo.test")
	sb.Where(sb.Equal("id", 1))

	// Consume the query in an outer INSERT ... SELECT.
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	ib.SelectBuilder(sb)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = ?
	// [1]
}

func TestInsertBuilderSelectBuilder(t *testing.T) {
	a := assert.New(t)

	// A partition query with a CTE, filters and args of its own.
	cte := With(
		CTEQuery("recent_orders").As(
			Select("id", "user_id", "status", "amount", "created_at").
				From("orders").
				Where("created_at >= '2026-01-01'"),
		),
	)
	sb := PostgreSQL.NewSelectBuilder().With(cte).
		Select("id", "user_id", "status", "amount").
		From("recent_orders")
	sb.Where(sb.Equal("status", "paid"))
	sb.Where(sb.Between("created_at", "2026-11-01", "2026-11-02"))

	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("report_orders").Cols("id", "user_id", "status", "amount")
	ib.SelectBuilder(sb)

	sql, args := ib.Build()
	a.Equal(sql, "INSERT INTO report_orders (id, user_id, status, amount) "+
		"WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status = $1 AND created_at BETWEEN $2 AND $3")
	a.Equal(args, []interface{}{"paid", "2026-11-01", "2026-11-02"})

	// The consumed query is compiled within the args scope of ib:
	// its placeholders are renumbered after ib's own args.
	ib2 := PostgreSQL.NewInsertBuilder()
	ib2.InsertInto("report_orders").Cols("id", "user_id", "status", "amount")
	ib2.SQL("/* " + ib2.Var("etl") + " */")
	ib2.SelectBuilder(sb)

	sql, args = ib2.Build()
	a.Equal(sql, "INSERT INTO report_orders (id, user_id, status, amount) /* $1 */ "+
		"WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status = $2 AND created_at BETWEEN $3 AND $4")
	a.Equal(args, []interface{}{"etl", "paid", "2026-11-01", "2026-11-02"})

	// The same query can be consumed by both inserts independently.
	a.Equal(sb.String(), "WITH recent_orders AS (SELECT id, user_id, status, amount, created_at FROM orders WHERE created_at >= '2026-01-01') "+
		"SELECT id, user_id, status, amount FROM recent_orders WHERE status = $1 AND created_at BETWEEN $2 AND $3")
}

func TestInsertBuilderSelectBuilderReplace(t *testing.T) {
	a := assert.New(t)

	sb1 := Select("id").From("t1")
	sb2 := Select("id").From("t2")

	// Only the builder set last is used.
	ib := NewInsertBuilder()
	ib.InsertInto("t").Cols("id")
	ib.SelectBuilder(sb1).SelectBuilder(sb2)
	a.Equal(ib.String(), "INSERT INTO t (id) SELECT id FROM t2")

	// SelectBuilder overrides Values, like Select does.
	ib = NewInsertBuilder()
	ib.InsertInto("t").Cols("id")
	ib.Values(1)
	ib.SelectBuilder(sb1)
	a.Equal(ib.String(), "INSERT INTO t (id) SELECT id FROM t1")

	// A nil builder removes the SELECT statement.
	ib = NewInsertBuilder()
	ib.InsertInto("t").Cols("id")
	ib.SelectBuilder(sb1)
	ib.SelectBuilder(nil)
	a.Equal(ib.String(), "INSERT INTO t (id)")
}

func ExampleInsertBuilder_NumValue() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	ib.Values(1, "Huan Du")
	ib.Values(2, "Charmy Liu")

	// Count the number of values.
	fmt.Println(ib.NumValue())

	// Output:
	// 2
}

func ExampleInsertBuilder_Returning() {
	sql, args := InsertInto("user").
		Cols("name").Values("Huan Du").
		Returning("id").
		BuildWithFlavor(PostgreSQL)

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO user (name) VALUES ($1) RETURNING id
	// [Huan Du]
}

func TestInsertBuilderReturning(test *testing.T) {
	a := assert.New(test)
	ib := InsertInto("user").
		Cols("name").Values("Huan Du").
		Returning("id")

	sql, _ := ib.BuildWithFlavor(MySQL)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(PostgreSQL)
	a.Equal("INSERT INTO user (name) VALUES ($1) RETURNING id", sql)

	sql, _ = ib.BuildWithFlavor(SQLite)
	a.Equal("INSERT INTO user (name) VALUES (?) RETURNING id", sql)

	sql, _ = ib.BuildWithFlavor(SQLServer)
	a.Equal("INSERT INTO user (name) OUTPUT INSERTED.id VALUES (@p1)", sql)

	sql, _ = ib.BuildWithFlavor(CQL)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(ClickHouse)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(Presto)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(Oracle)
	a.Equal("INSERT INTO user (name) VALUES (:1)", sql)

	sql, _ = ib.BuildWithFlavor(Informix)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)
}

func TestInsertBuilderGetFlavor(t *testing.T) {
	a := assert.New(t)
	ib := newInsertBuilder()

	ib.SetFlavor(PostgreSQL)
	flavor := ib.Flavor()
	a.Equal(PostgreSQL, flavor)

	ibClick := ClickHouse.NewInsertBuilder()
	flavor = ibClick.Flavor()
	a.Equal(ClickHouse, flavor)
}

func TestIssue200(t *testing.T) {
	a := assert.New(t)
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertIgnoreInto("table")
	ib.Cols("data")
	sb := ib.Select("id", "data").From("table")
	sb.Where(sb.Equal("id", 1))

	query, _ := ib.Build()
	a.Equal(query, "INSERT INTO table (data) SELECT id, data FROM table WHERE id = $1 ON CONFLICT DO NOTHING")
}

func TestInsertBuilderClone(t *testing.T) {
	a := assert.New(t)

	ib := InsertInto("demo.user").Cols("id", "name")
	ib.Values(1, "A")

	clone := ib.Clone()

	s1, args1 := ib.Build()
	s2, args2 := clone.Build()
	a.Equal(s1, s2)
	a.Equal(args1, args2)

	// mutate clone and verify original unchanged
	clone.Values(2, "B")
	a.NotEqual(ib.String(), clone.String())
}
