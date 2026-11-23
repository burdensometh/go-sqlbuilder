// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"strings"
)

// Direction suffixes recognized when parsing ORDER BY columns
// for keyset pagination.
const (
	keysetAscSuffix  = " asc"
	keysetDescSuffix = " desc"
)

// keysetColumn is an ORDER BY column with its sort direction,
// used to build a keyset pagination continuation condition.
type keysetColumn struct {
	expr string // The column or expression as it appears in ORDER BY, without the ASC/DESC suffix.
	desc bool   // True for DESC; false for ASC.
}

// SeekAfter adds a keyset pagination condition (a.k.a. the seek method or a
// continuation position) to the WHERE clause. The condition matches all rows
// that come strictly after the row identified by values in the ORDER BY
// order, so a caller can page through a large result set without OFFSET:
// run the query with a LIMIT, then call SeekAfter with the ORDER BY column
// values of the last row of the current page to fetch the next page, and
// repeat until a page returns fewer rows than the limit (or no rows at all)
// to read to the end.
//
// The values are bound to the ORDER BY columns positionally, so the number
// of values must equal the number of ORDER BY columns. The sort direction of
// every column is honored:
//
//   - For an ASC column, rows after the position have a greater value.
//   - For a DESC column, rows after the position have a smaller value.
//
// Ties are broken lexicographically: when leading columns compare equal, the
// following columns decide. To make the continuation position exact, the
// ORDER BY columns as a whole must be unique — the idiomatic pattern is to
// sort by a non-unique column first (e.g. updated_at, which many rows can
// share within the same second) and append a unique column (e.g. the primary
// key id) as the tie breaker:
//
//	sb.OrderByAsc("updated_at").OrderByAsc("id")
//	sb.SeekAfter(lastUpdatedAt, lastID)
//
// Because the comparison is strict on the full tuple, the row identified by
// values itself is excluded: with a unique ORDER BY tuple, consecutive pages
// never overlap and never skip rows, even when many rows share the same
// leading column values.
//
// When all ORDER BY columns share the same direction and the flavor supports
// row value comparison, the condition is built as a row value comparison,
// e.g. "(updated_at, id) > (?, ?)". Otherwise — for flavors without row
// value comparison (e.g. SQLServer, CQL, Informix, Doris) or for mixed sort
// directions — the same key range is selected by an equivalent expanded
// OR/AND condition, e.g. "(updated_at > ?) OR (updated_at = ? AND id > ?)".
//
// Note that keyset columns are compared with NULL-unaware operators; the
// ORDER BY columns should be NOT NULL for the continuation to be exact.
//
// SeekAfter panics if no ORDER BY column is set or the number of values
// doesn't match the number of ORDER BY columns. Call it at most once per
// builder; use Clone to derive the next page's builder from a shared base.
func (sb *SelectBuilder) SeekAfter(values ...interface{}) *SelectBuilder {
	return sb.seek("SeekAfter", true, values)
}

// SeekBefore is the mirror of SeekAfter: it adds a keyset pagination
// condition matching all rows that come strictly before the row identified
// by values in the ORDER BY order (greater values for DESC columns, smaller
// values for ASC columns). It can be used to read pages in reverse.
//
// See SeekAfter for the direction and tie-breaking strategy, the row value
// comparison fallback and the panics.
func (sb *SelectBuilder) SeekBefore(values ...interface{}) *SelectBuilder {
	return sb.seek("SeekBefore", false, values)
}

func (sb *SelectBuilder) seek(name string, after bool, values []interface{}) *SelectBuilder {
	cols := sb.keysetColumns()

	if len(cols) == 0 {
		panic(fmt.Errorf("sqlbuilder: %s requires ORDER BY columns; declare the sort order with OrderByAsc/OrderByDesc (or OrderBy) first", name))
	}

	if len(values) != len(cols) {
		panic(fmt.Errorf("sqlbuilder: %s requires exactly %d value(s) to match the %d ORDER BY column(s), got %d", name, len(cols), len(cols), len(values)))
	}

	// The condition is built at compile time so that the decision between a
	// row value comparison and the expanded OR/AND form honors the flavor
	// used in Build/BuildWithFlavor.
	expr := sb.Var(condBuilder{
		Builder: func(ctx *argsCompileContext) {
			buildKeysetCondition(ctx, cols, values, after)
		},
	})
	sb.Where(expr)
	return sb
}

// keysetColumns returns the ORDER BY columns of sb with their sort directions.
//
// Columns declared by OrderByAsc and OrderByDesc carry an explicit direction.
// For columns declared by OrderBy, a trailing "ASC"/"DESC" (case-insensitive)
// in the column string is honored; otherwise the direction defaults to the
// direction set by Asc/Desc, or ASC if none was set.
func (sb *SelectBuilder) keysetColumns() []keysetColumn {
	cols := make([]keysetColumn, 0, len(sb.orderByCols))

	for _, raw := range sb.orderByCols {
		expr, desc := splitKeysetColumn(raw, sb.order)

		if expr == "" {
			continue
		}

		cols = append(cols, keysetColumn{expr: expr, desc: desc})
	}

	return cols
}

// splitKeysetColumn splits a raw ORDER BY column string into the column
// expression and its sort direction. A trailing " ASC"/" DESC" suffix
// (case-insensitive) overrides the defaultOrder, which is the direction set
// by Asc/Desc and may be empty (meaning ASC).
func splitKeysetColumn(raw, defaultOrder string) (expr string, desc bool) {
	expr = strings.TrimSpace(raw)

	if n := len(expr); n > len(keysetDescSuffix) && strings.EqualFold(expr[n-len(keysetDescSuffix):], keysetDescSuffix) {
		return strings.TrimSpace(expr[:n-len(keysetDescSuffix)]), true
	}

	if n := len(expr); n > len(keysetAscSuffix) && strings.EqualFold(expr[n-len(keysetAscSuffix):], keysetAscSuffix) {
		return strings.TrimSpace(expr[:n-len(keysetAscSuffix)]), false
	}

	return expr, strings.EqualFold(defaultOrder, "DESC")
}

// keysetSupportsRowComparison reports whether flavor supports row value
// comparison like "(a, b) > (1, 2)" in WHERE.
//
// SQLServer, CQL, Informix and Doris don't support row value comparison;
// for them the expanded OR/AND form, which selects exactly the same key
// range, is used instead.
func keysetSupportsRowComparison(flavor Flavor) bool {
	switch flavor {
	case MySQL, PostgreSQL, SQLite, ClickHouse, Presto, Oracle:
		return true
	}

	return false
}

// buildKeysetCondition writes the keyset continuation condition to ctx.
//
// The condition selects all rows sorted strictly after (or before, when
// after is false) the position identified by values in the ORDER BY order
// defined by cols. The comparison is lexicographic: ties on leading columns
// are broken by the following columns.
func buildKeysetCondition(ctx *argsCompileContext, cols []keysetColumn, values []interface{}, after bool) {
	ops := make([]string, len(cols))
	uniform := true

	for i, col := range cols {
		// For an ASC column, rows after the position have a greater value
		// and rows before it have a smaller one; DESC is the opposite.
		if col.desc == after {
			ops[i] = "<"
		} else {
			ops[i] = ">"
		}

		if ops[i] != ops[0] {
			uniform = false
		}
	}

	// A single column needs no tuple at all.
	if len(cols) == 1 {
		ctx.WriteString(cols[0].expr)
		ctx.WriteString(" ")
		ctx.WriteString(ops[0])
		ctx.WriteString(" ")
		ctx.WriteValue(values[0])
		return
	}

	// A row value comparison like "(a, b) > (1, 2)" is lexicographic with
	// the same operator applied to all columns, so it can express the
	// continuation condition only when all columns share the same direction.
	// Mixed directions always use the expanded form.
	if uniform && keysetSupportsRowComparison(ctx.Flavor) {
		ctx.WriteString("(")

		for i, col := range cols {
			if i > 0 {
				ctx.WriteString(", ")
			}

			ctx.WriteString(col.expr)
		}

		ctx.WriteString(") ")
		ctx.WriteString(ops[0])
		ctx.WriteString(" (")
		ctx.WriteValues(values, ", ")
		ctx.WriteString(")")
		return
	}

	// Expanded form selecting the same key range:
	//
	//	(c1 op1 v1) OR (c1 = v1 AND c2 op2 v2) OR (c1 = v1 AND c2 = v2 AND c3 op3 v3) ...
	//
	// The whole condition is parenthesized so that it can be safely combined
	// with other WHERE expressions using AND.
	ctx.WriteString("(")

	for i := range cols {
		if i > 0 {
			ctx.WriteString(" OR ")
		}

		ctx.WriteString("(")

		for j := 0; j < i; j++ {
			ctx.WriteString(cols[j].expr)
			ctx.WriteString(" = ")
			ctx.WriteValue(values[j])
			ctx.WriteString(" AND ")
		}

		ctx.WriteString(cols[i].expr)
		ctx.WriteString(" ")
		ctx.WriteString(ops[i])
		ctx.WriteString(" ")
		ctx.WriteValue(values[i])
		ctx.WriteString(")")
	}

	ctx.WriteString(")")
}
