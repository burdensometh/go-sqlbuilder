// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"sort"
	"testing"

	"github.com/huandu/go-assert"
)

func TestSeekAfterRowComparison(t *testing.T) {
	a := assert.New(t)

	// All ASC: row value comparison with '>'.
	sb := MySQL.NewSelectBuilder()
	sb.Select("id", "updated_at").From("transactions")
	sb.OrderByAsc("updated_at").OrderByAsc("id")
	sb.SeekAfter(1700000000, 42)

	sql, args := sb.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE (updated_at, id) > (?, ?) ORDER BY updated_at ASC, id ASC")
	a.Equal(args, []interface{}{1700000000, 42})

	// All DESC: row value comparison with '<'.
	sb = MySQL.NewSelectBuilder()
	sb.Select("id", "updated_at").From("transactions")
	sb.OrderByDesc("updated_at").OrderByDesc("id")
	sb.SeekAfter(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE (updated_at, id) < (?, ?) ORDER BY updated_at DESC, id DESC")
	a.Equal(args, []interface{}{1700000000, 42})

	// The deprecated OrderBy + Desc combination is honored as well.
	sb = MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderBy("updated_at", "id").Desc()
	sb.SeekAfter(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE (updated_at, id) < (?, ?) ORDER BY updated_at, id DESC")
	a.Equal(args, []interface{}{1700000000, 42})

	// Direction suffixes in raw OrderBy columns are parsed case-insensitively.
	sb = MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderBy("updated_at DESC", "id desc")
	sb.SeekAfter(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE (updated_at, id) < (?, ?) ORDER BY updated_at DESC, id desc")
	a.Equal(args, []interface{}{1700000000, 42})
}

func TestSeekAfterExpandedForm(t *testing.T) {
	a := assert.New(t)

	// Mixed directions always use the expanded OR/AND form, even when the
	// flavor supports row value comparison.
	sb := MySQL.NewSelectBuilder()
	sb.Select("id", "updated_at").From("transactions")
	sb.OrderByDesc("updated_at").OrderByAsc("id")
	sb.SeekAfter(1700000000, 42)

	sql, args := sb.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE ((updated_at < ?) OR (updated_at = ? AND id > ?)) ORDER BY updated_at DESC, id ASC")
	a.Equal(args, []interface{}{1700000000, 1700000000, 42})

	// Flavors without row value comparison use the expanded form selecting
	// the same key range.
	sb = SQLServer.NewSelectBuilder()
	sb.Select("id", "updated_at").From("transactions")
	sb.OrderByAsc("updated_at").OrderByAsc("id")
	sb.SeekAfter(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE ((updated_at > @p1) OR (updated_at = @p2 AND id > @p3)) ORDER BY updated_at ASC, id ASC")
	a.Equal(args, []interface{}{1700000000, 1700000000, 42})

	// Three columns: ties are broken lexicographically.
	sb = CQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByAsc("updated_at").OrderByDesc("amount").OrderByAsc("id")
	sb.SeekAfter(1700000000, 100, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE ((updated_at > ?) OR (updated_at = ? AND amount < ?) OR (updated_at = ? AND amount = ? AND id > ?)) ORDER BY updated_at ASC, amount DESC, id ASC")
	a.Equal(args, []interface{}{1700000000, 1700000000, 100, 1700000000, 100, 42})
}

func TestSeekAfterFlavors(t *testing.T) {
	a := assert.New(t)

	cases := []struct {
		flavor Flavor
		where  string
		args   []interface{}
	}{
		{MySQL, "(updated_at, id) > (?, ?)", []interface{}{1, 2}},
		{PostgreSQL, "(updated_at, id) > ($1, $2)", []interface{}{1, 2}},
		{SQLite, "(updated_at, id) > (?, ?)", []interface{}{1, 2}},
		{ClickHouse, "(updated_at, id) > (?, ?)", []interface{}{1, 2}},
		{Presto, "(updated_at, id) > (?, ?)", []interface{}{1, 2}},
		{Oracle, "(updated_at, id) > (:1, :2)", []interface{}{1, 2}},
		{SQLServer, "((updated_at > @p1) OR (updated_at = @p2 AND id > @p3))", []interface{}{1, 1, 2}},
		{CQL, "((updated_at > ?) OR (updated_at = ? AND id > ?))", []interface{}{1, 1, 2}},
		{Informix, "((updated_at > ?) OR (updated_at = ? AND id > ?))", []interface{}{1, 1, 2}},
		{Doris, "((updated_at > ?) OR (updated_at = ? AND id > ?))", []interface{}{1, 1, 2}},
	}

	for _, c := range cases {
		sb := c.flavor.NewSelectBuilder()
		sb.Select("id").From("transactions")
		sb.OrderByAsc("updated_at").OrderByAsc("id")
		sb.SeekAfter(1, 2)

		sql, args := sb.Build()
		a.Equal(sql, "SELECT id FROM transactions WHERE "+c.where+" ORDER BY updated_at ASC, id ASC")
		a.Equal(args, c.args)
	}

	// The choice between a row value comparison and the expanded form is
	// made at compile time, so the same builder can serve both flavors.
	sb := NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByAsc("updated_at").OrderByAsc("id")
	sb.SeekAfter(1, 2)

	sql, args := sb.BuildWithFlavor(PostgreSQL)
	a.Equal(sql, "SELECT id FROM transactions WHERE (updated_at, id) > ($1, $2) ORDER BY updated_at ASC, id ASC")
	a.Equal(args, []interface{}{1, 2})

	sql, args = sb.BuildWithFlavor(SQLServer)
	a.Equal(sql, "SELECT id FROM transactions WHERE ((updated_at > @p1) OR (updated_at = @p2 AND id > @p3)) ORDER BY updated_at ASC, id ASC")
	a.Equal(args, []interface{}{1, 1, 2})
}

func TestSeekBefore(t *testing.T) {
	a := assert.New(t)

	// SeekBefore mirrors the operators: smaller values for ASC columns,
	// greater values for DESC columns.
	sb := MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByAsc("updated_at").OrderByAsc("id")
	sb.SeekBefore(1700000000, 42)

	sql, args := sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE (updated_at, id) < (?, ?) ORDER BY updated_at ASC, id ASC")
	a.Equal(args, []interface{}{1700000000, 42})

	sb = MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByDesc("updated_at").OrderByDesc("id")
	sb.SeekBefore(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE (updated_at, id) > (?, ?) ORDER BY updated_at DESC, id DESC")
	a.Equal(args, []interface{}{1700000000, 42})

	sb = MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByAsc("updated_at").OrderByDesc("id")
	sb.SeekBefore(1700000000, 42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE ((updated_at < ?) OR (updated_at = ? AND id > ?)) ORDER BY updated_at ASC, id DESC")
	a.Equal(args, []interface{}{1700000000, 1700000000, 42})
}

func TestSeekSingleColumn(t *testing.T) {
	a := assert.New(t)

	sb := MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByAsc("id")
	sb.SeekAfter(42)

	sql, args := sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE id > ? ORDER BY id ASC")
	a.Equal(args, []interface{}{42})

	sb = MySQL.NewSelectBuilder()
	sb.Select("id").From("transactions")
	sb.OrderByDesc("id")
	sb.SeekAfter(42)

	sql, args = sb.Build()
	a.Equal(sql, "SELECT id FROM transactions WHERE id < ? ORDER BY id DESC")
	a.Equal(args, []interface{}{42})
}

func TestSeekCombinedWithWhere(t *testing.T) {
	a := assert.New(t)

	// The keyset condition is ANDed with other WHERE expressions; the
	// expanded form is parenthesized as a whole to keep the key range intact.
	sb := MySQL.NewSelectBuilder()
	sb.Select("id", "updated_at").From("transactions")
	sb.Where(
		sb.GE("updated_at", 1699999999),
		sb.Equal("status", "settled"),
	)
	sb.OrderByDesc("updated_at").OrderByAsc("id")
	sb.SeekAfter(1700000000, 42)
	sb.Limit(500)

	sql, args := sb.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE updated_at >= ? AND status = ? AND ((updated_at < ?) OR (updated_at = ? AND id > ?)) ORDER BY updated_at DESC, id ASC LIMIT ?")
	a.Equal(args, []interface{}{1699999999, "settled", 1700000000, 1700000000, 42, 500})
}

func TestSeekPagingWithClone(t *testing.T) {
	a := assert.New(t)

	// The idiomatic paging loop: keep a base builder, clone it per page and
	// continue strictly after the last row of the previous page.
	base := MySQL.NewSelectBuilder()
	base.Select("id", "updated_at").From("transactions")
	base.OrderByAsc("updated_at").OrderByAsc("id")
	base.Limit(500)

	page1 := base.Clone()
	sql, args := page1.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions ORDER BY updated_at ASC, id ASC LIMIT ?")
	a.Equal(args, []interface{}{500})

	page2 := base.Clone()
	page2.SeekAfter(1700000000, 42)
	sql, args = page2.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions WHERE (updated_at, id) > (?, ?) ORDER BY updated_at ASC, id ASC LIMIT ?")
	a.Equal(args, []interface{}{1700000000, 42, 500})

	// The base builder is not polluted by the continuation condition.
	sql, _ = base.Build()
	a.Equal(sql, "SELECT id, updated_at FROM transactions ORDER BY updated_at ASC, id ASC LIMIT ?")
}

func TestSeekPanics(t *testing.T) {
	a := assert.New(t)

	seekWithoutOrderBy := func() (panicked bool) {
		defer func() {
			panicked = recover() != nil
		}()

		sb := MySQL.NewSelectBuilder()
		sb.Select("id").From("transactions")
		sb.SeekAfter(1, 2)
		return
	}

	seekWithWrongValueCount := func() (panicked bool) {
		defer func() {
			panicked = recover() != nil
		}()

		sb := MySQL.NewSelectBuilder()
		sb.Select("id").From("transactions")
		sb.OrderByAsc("updated_at").OrderByAsc("id")
		sb.SeekAfter(1)
		return
	}

	seekBeforeWithWrongValueCount := func() (panicked bool) {
		defer func() {
			panicked = recover() != nil
		}()

		sb := MySQL.NewSelectBuilder()
		sb.Select("id").From("transactions")
		sb.OrderByAsc("id")
		sb.SeekBefore(1, 2)
		return
	}

	a.Assert(seekWithoutOrderBy())
	a.Assert(seekWithWrongValueCount())
	a.Assert(seekBeforeWithWrongValueCount())
}

// keysetTestRow is a row of the synthetic transactions table used in the
// paging simulation below.
type keysetTestRow struct {
	updatedAt int64
	amount    int
	id        int
}

// keysetLess compares two rows in the ORDER BY order defined by cols.
// cols holds column indices: 0 = updatedAt, 1 = amount, 2 = id.
func keysetLess(cols []keysetColumn, a, b keysetTestRow) bool {
	va := []int64{a.updatedAt, int64(a.amount), int64(a.id)}
	vb := []int64{b.updatedAt, int64(b.amount), int64(b.id)}

	for _, col := range cols {
		idx := int(col.expr[0] - '0')

		if va[idx] == vb[idx] {
			continue
		}

		if col.desc {
			return va[idx] > vb[idx]
		}

		return va[idx] < vb[idx]
	}

	return false
}

// keysetAfterPred returns a predicate telling whether row comes strictly
// after the position identified by values. It mirrors the lexicographic
// semantics of the SQL condition built by buildKeysetCondition.
func keysetAfterPred(cols []keysetColumn, values []int64) func(keysetTestRow) bool {
	return func(row keysetTestRow) bool {
		v := []int64{row.updatedAt, int64(row.amount), int64(row.id)}

		for i, col := range cols {
			idx := int(col.expr[0] - '0')

			if v[idx] == values[i] {
				continue
			}

			if col.desc {
				return v[idx] < values[i]
			}

			return v[idx] > values[i]
		}

		return false
	}
}

func TestSeekPagingSimulation(t *testing.T) {
	a := assert.New(t)

	// Build a synthetic transactions table with heavy ties on updated_at:
	// many transactions share the same second, which is exactly where
	// OFFSET or single-column pagination would duplicate or skip rows.
	rows := make([]keysetTestRow, 0, 120)
	id := 1

	for sec := int64(1700000000); sec < 1700000010; sec++ {
		for j := 0; j < 12; j++ {
			rows = append(rows, keysetTestRow{
				updatedAt: sec,
				amount:    (j * 7) % 5, // Deliberately non-unique as well.
				id:        id,
			})
			id++
		}
	}

	cols := []keysetColumn{
		{expr: "0", desc: false}, // updated_at ASC
		{expr: "1", desc: true},  // amount DESC
		{expr: "2", desc: false}, // id ASC
	}

	sort.Slice(rows, func(i, j int) bool {
		return keysetLess(cols, rows[i], rows[j])
	})

	// Page through the sorted rows with the same strict lexicographic
	// continuation that SeekAfter generates in SQL.
	const pageSize = 7

	var (
		seen    []keysetTestRow
		cursor  []int64
		pageNum int
	)

	for {
		var page []keysetTestRow

		if cursor == nil {
			page = append(page, rows[:minInt(pageSize, len(rows))]...)
		} else {
			pred := keysetAfterPred(cols, cursor)

			for _, row := range rows {
				if pred(row) {
					page = append(page, row)

					if len(page) == pageSize {
						break
					}
				}
			}
		}

		if len(page) == 0 {
			break
		}

		seen = append(seen, page...)
		last := page[len(page)-1]
		cursor = []int64{last.updatedAt, int64(last.amount), int64(last.id)}
		pageNum++

		// A short page means the end of the result set is reached.
		if len(page) < pageSize {
			break
		}
	}

	// Paging to the end yields every row exactly once, in ORDER BY order.
	a.Equal(len(seen), len(rows))
	a.Equal(seen, rows)
	a.Assert(pageNum > 1)
}

func TestSeekExpandedEqualsRowComparison(t *testing.T) {
	a := assert.New(t)

	// The expanded OR/AND form and the row value comparison select exactly
	// the same key range. Verify the equivalence on every possible cursor
	// position of a small dataset with ties, for both uniform directions.
	dataset := []keysetTestRow{
		{updatedAt: 1, id: 1},
		{updatedAt: 1, id: 2},
		{updatedAt: 1, id: 3},
		{updatedAt: 2, id: 4},
		{updatedAt: 2, id: 5},
		{updatedAt: 3, id: 6},
	}

	// expanded simulates the OR/AND chain
	// "(updated_at > t) OR (updated_at = t AND id > i)".
	expanded := func(row keysetTestRow, t, i int64, desc bool) bool {
		if desc {
			return row.updatedAt < t || (row.updatedAt == t && int64(row.id) < i)
		}

		return row.updatedAt > t || (row.updatedAt == t && int64(row.id) > i)
	}

	// rowValue simulates the row value comparison "(updated_at, id) > (t, i)"
	// as a single lexicographic comparison of the two tuples.
	rowValue := func(row keysetTestRow, t, i int64, desc bool) bool {
		cmp := row.updatedAt - t

		if cmp == 0 {
			cmp = int64(row.id) - i
		}

		if desc {
			return cmp < 0
		}

		return cmp > 0
	}

	for _, desc := range []bool{false, true} {
		for _, cursorRow := range dataset {
			t, i := cursorRow.updatedAt, int64(cursorRow.id)

			for _, row := range dataset {
				a.Equal(expanded(row, t, i, desc), rowValue(row, t, i, desc))
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func ExampleSelectBuilder_SeekAfter() {
	// Export transactions page by page, ordered by updated_at. Many
	// transactions can share the same updated_at second, so the unique id
	// is appended as the tie breaker of the continuation position.
	base := MySQL.NewSelectBuilder()
	base.Select("id", "updated_at", "amount").From("transactions")
	base.OrderByAsc("updated_at").OrderByAsc("id")
	base.Limit(500)

	// Page 1: no continuation position yet.
	sql, args := base.Clone().Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Page 2: continue strictly after the last row of page 1, identified
	// by its (updated_at, id) values.
	page2 := base.Clone()
	page2.SeekAfter(1700000000, 42)
	sql, args = page2.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT id, updated_at, amount FROM transactions ORDER BY updated_at ASC, id ASC LIMIT ?
	// [500]
	// SELECT id, updated_at, amount FROM transactions WHERE (updated_at, id) > (?, ?) ORDER BY updated_at ASC, id ASC LIMIT ?
	// [1700000000 42 500]
}
