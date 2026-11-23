// Copyright 2026 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	rowNumberMarkerInit injectionMarker = iota
	rowNumberMarkerAfterSelect
	rowNumberMarkerAfterFrom
	rowNumberMarkerAfterWhere
	rowNumberMarkerAfterOrderBy
	rowNumberMarkerAfterLimit
)

// Default alias of the ROW_NUMBER ranking column and the inner derived table.
const (
	defaultRowNumberCol   = "rn"
	defaultRowNumberAlias = "t"
)

// NewRowNumberBuilder creates a new ROW_NUMBER builder.
func NewRowNumberBuilder() *RowNumberBuilder {
	return DefaultFlavor.NewRowNumberBuilder()
}

func newRowNumberBuilder() *RowNumberBuilder {
	return &RowNumberBuilder{
		args:         &Args{},
		injection:    newInjection(),
		rowNumberCol: defaultRowNumberCol,
		derivedAlias: defaultRowNumberAlias,
	}
}

// Clone returns a deep copy of RowNumberBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (rnb *RowNumberBuilder) Clone() *RowNumberBuilder {
	return clone.Clone(rnb).(*RowNumberBuilder)
}

func init() {
	t := reflect.TypeOf(RowNumberBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		rnb := cloned.Addr().Interface().(*RowNumberBuilder)

		if rnb.source != nil {
			rnb.args.Replace(rnb.sourceVar, rnb.source)
		}
	})
}

// RowNumberBuilder is a builder that picks the first row in every group defined
// by a composite business key.
//
// It wraps a source SELECT with a ROW_NUMBER() window function. The
// PartitionBy columns define the composite business key (the PARTITION BY
// clause), and the OrderBy columns define an explicit total order inside every
// partition. Rows ranked 1 are kept, all other rows are filtered out.
//
// The generated SQL has two layers so that the source query is never modified:
//
//	SELECT <cols> FROM (
//	    SELECT ROW_NUMBER() OVER (PARTITION BY ... ORDER BY ...) AS rn, <source columns>
//	    FROM (<source sql>) AS t
//	) AS t WHERE rn = 1
//
// The builder implements Builder, so it can be nested into another SELECT or
// used directly as the source of INSERT INTO ... SELECT ...
type RowNumberBuilder struct {
	source    Builder
	sourceVar string

	partitionCols []string
	orderByCols   []string

	selectCols   []string
	rowNumberCol string
	derivedAlias string

	outerOrderByCols []string
	limitVar         string
	offsetVar        string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(RowNumberBuilder)

// From sets the source query whose rows are ranked and deduplicated.
// The source builder is embedded as a subquery and is never modified.
func (rnb *RowNumberBuilder) From(source Builder) *RowNumberBuilder {
	rnb.source = source
	rnb.sourceVar = rnb.args.Add(source)
	rnb.marker = rowNumberMarkerAfterFrom
	return rnb
}

// PartitionBy sets the columns of the composite business key, i.e. the
// PARTITION BY clause of the window function. Rows sharing the same key values
// form one partition and exactly one of them is kept.
//
// Columns are expressions evaluated against the source rows, so they may use
// helpers such as NullIfEmpty to normalize empty keys.
func (rnb *RowNumberBuilder) PartitionBy(col ...string) *RowNumberBuilder {
	rnb.partitionCols = col
	return rnb
}

// OrderBy adds columns to the ORDER BY clause of the window function.
// Direction must be encoded in each expression, e.g. "updated_at DESC".
//
// It's recommended to use OrderByAsc or OrderByDesc instead for better readability.
func (rnb *RowNumberBuilder) OrderBy(col ...string) *RowNumberBuilder {
	rnb.orderByCols = append(rnb.orderByCols, col...)
	rnb.marker = rowNumberMarkerAfterOrderBy
	return rnb
}

// OrderByAsc adds a column to the window ORDER BY clause with ASC direction.
// It supports chaining multiple calls to add several ORDER BY columns with
// different directions.
func (rnb *RowNumberBuilder) OrderByAsc(col string) *RowNumberBuilder {
	rnb.orderByCols = append(rnb.orderByCols, col+" ASC")
	rnb.marker = rowNumberMarkerAfterOrderBy
	return rnb
}

// OrderByDesc adds a column to the window ORDER BY clause with DESC direction.
// It supports chaining multiple calls to add several ORDER BY columns with
// different directions.
func (rnb *RowNumberBuilder) OrderByDesc(col string) *RowNumberBuilder {
	rnb.orderByCols = append(rnb.orderByCols, col+" DESC")
	rnb.marker = rowNumberMarkerAfterOrderBy
	return rnb
}

// TieBreaker appends a unique column to the window ORDER BY clause with ASC
// direction to guarantee a total order.
//
// ROW_NUMBER assigns distinct ranks even when business sort columns are equal,
// but the winner among equal rows is unspecified without a deterministic final
// sort key. Call this method with a unique column (typically the primary key)
// after all business sort columns so that equal rows always resolve to the same
// winner and results are stable across runs.
func (rnb *RowNumberBuilder) TieBreaker(col string) *RowNumberBuilder {
	return rnb.OrderByAsc(col)
}

// Select sets the columns projected by the outer query.
// When no column is set, "*" is projected so that every source column, including
// the ranking column, is available for INSERT INTO ... SELECT ....
func (rnb *RowNumberBuilder) Select(col ...string) *RowNumberBuilder {
	rnb.selectCols = col
	rnb.marker = rowNumberMarkerAfterSelect
	return rnb
}

// RowNumberCol overrides the alias of the ROW_NUMBER ranking column.
// The default alias is "rn".
func (rnb *RowNumberBuilder) RowNumberCol(alias string) *RowNumberBuilder {
	if alias != "" {
		rnb.rowNumberCol = alias
	}

	return rnb
}

// DerivedAlias overrides the alias of the inner derived table.
// The default alias is "t".
func (rnb *RowNumberBuilder) DerivedAlias(alias string) *RowNumberBuilder {
	if alias != "" {
		rnb.derivedAlias = alias
	}

	return rnb
}

// OrderResultBy adds columns to the ORDER BY clause of the outer query.
// It only changes the order in which the deduplicated rows are returned and
// never affects which row is kept inside a partition.
func (rnb *RowNumberBuilder) OrderResultBy(col ...string) *RowNumberBuilder {
	rnb.outerOrderByCols = append(rnb.outerOrderByCols, col...)
	return rnb
}

// Limit sets the LIMIT of the outer query.
func (rnb *RowNumberBuilder) Limit(limit int) *RowNumberBuilder {
	if limit < 0 {
		rnb.limitVar = ""
		return rnb
	}

	rnb.limitVar = rnb.Var(limit)
	rnb.marker = rowNumberMarkerAfterLimit
	return rnb
}

// Offset sets the OFFSET of the outer query.
func (rnb *RowNumberBuilder) Offset(offset int) *RowNumberBuilder {
	if offset < 0 {
		rnb.offsetVar = ""
		return rnb
	}

	rnb.offsetVar = rnb.Var(offset)
	rnb.marker = rowNumberMarkerAfterLimit
	return rnb
}

// String returns the compiled SELECT string.
func (rnb *RowNumberBuilder) String() string {
	s, _ := rnb.Build()
	return s
}

// Build returns compiled SELECT string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (rnb *RowNumberBuilder) Build() (sql string, args []interface{}) {
	return rnb.BuildWithFlavor(rnb.args.Flavor)
}

// BuildWithFlavor returns compiled SELECT string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
//
// It panics when the source builder is missing or no ORDER BY column is set,
// as such a query cannot rank rows deterministically.
func (rnb *RowNumberBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	rnb.validate()

	buf := newStringBuilder()
	rnb.injection.WriteTo(buf, rowNumberMarkerInit)

	// Outer query: project the requested output columns and keep only rank-1 rows.
	buf.WriteLeadingString("SELECT ")
	if len(rnb.selectCols) > 0 {
		buf.WriteStrings(rnb.selectCols, ", ")
	} else {
		buf.WriteRune('*')
	}

	rnb.injection.WriteTo(buf, rowNumberMarkerAfterSelect)

	buf.WriteLeadingString("FROM (")

	// Inner query: source columns plus the ROW_NUMBER ranking column.
	buf.WriteString("SELECT ROW_NUMBER() OVER (")

	if len(rnb.partitionCols) > 0 {
		buf.WriteString("PARTITION BY ")
		buf.WriteStrings(rnb.partitionCols, ", ")
		buf.WriteRune(' ')
	}

	buf.WriteString("ORDER BY ")
	buf.WriteStrings(rnb.orderByCols, ", ")
	buf.WriteString(") AS ")
	buf.WriteString(rnb.rowNumberCol)
	buf.WriteString(", * FROM (")
	buf.WriteString(rnb.sourceVar)
	buf.WriteString(") AS ")
	buf.WriteString(rnb.derivedAlias)

	buf.WriteRune(')')
	buf.WriteLeadingString("AS ")
	buf.WriteString(rnb.derivedAlias)

	rnb.injection.WriteTo(buf, rowNumberMarkerAfterFrom)

	buf.WriteLeadingString("WHERE ")
	buf.WriteString(rnb.rowNumberCol)
	buf.WriteString(" = 1")

	rnb.injection.WriteTo(buf, rowNumberMarkerAfterWhere)

	if len(rnb.outerOrderByCols) > 0 {
		buf.WriteLeadingString("ORDER BY ")
		buf.WriteStrings(rnb.outerOrderByCols, ", ")
	}

	rnb.injection.WriteTo(buf, rowNumberMarkerAfterOrderBy)

	if len(rnb.limitVar) > 0 || len(rnb.offsetVar) > 0 {
		rnb.writeLimitOffset(buf, flavor)
		rnb.injection.WriteTo(buf, rowNumberMarkerAfterLimit)
	}

	return rnb.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

func (rnb *RowNumberBuilder) writeLimitOffset(buf *stringBuilder, flavor Flavor) {
	switch flavor {
	case PostgreSQL, MySQL, SQLite, ClickHouse:
		if len(rnb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(rnb.limitVar)
		}

		if len(rnb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(rnb.offsetVar)
		}

	case Presto:
		if len(rnb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(rnb.offsetVar)
		}

		if len(rnb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(rnb.limitVar)
		}

	case SQLServer, Oracle:
		if len(rnb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(rnb.offsetVar)
			buf.WriteString(" ROWS")
		} else {
			buf.WriteLeadingString("OFFSET 0 ROWS")
		}

		if len(rnb.limitVar) > 0 {
			buf.WriteLeadingString("FETCH NEXT ")
			buf.WriteString(rnb.limitVar)
			buf.WriteString(" ROWS ONLY")
		}

	case Informix:
		if len(rnb.offsetVar) > 0 {
			buf.WriteLeadingString("SKIP ")
			buf.WriteString(rnb.offsetVar)
		}

		if len(rnb.limitVar) > 0 {
			buf.WriteLeadingString("FIRST ")
			buf.WriteString(rnb.limitVar)
		}

	case Doris:
		if len(rnb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(fmt.Sprint(rnb.args.Value(rnb.limitVar)))
		}

		if len(rnb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(fmt.Sprint(rnb.args.Value(rnb.offsetVar)))
		}
	}
}

func (rnb *RowNumberBuilder) validate() {
	if rnb.source == nil {
		panic("go-sqlbuilder: RowNumberBuilder requires a source builder set by From")
	}

	if len(rnb.orderByCols) == 0 {
		panic("go-sqlbuilder: RowNumberBuilder requires at least one ORDER BY column")
	}
}

// SetFlavor sets the flavor of compiled sql.
func (rnb *RowNumberBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = rnb.args.Flavor
	rnb.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder.
func (rnb *RowNumberBuilder) Flavor() Flavor {
	return rnb.args.Flavor
}

// Var returns a placeholder for value.
func (rnb *RowNumberBuilder) Var(arg interface{}) string {
	return rnb.args.Add(arg)
}

// SQL adds an arbitrary sql to current position.
func (rnb *RowNumberBuilder) SQL(sql string) *RowNumberBuilder {
	rnb.injection.SQL(rnb.marker, sql)
	return rnb
}

// NullIfEmpty normalizes an empty string to NULL so that NULL and the empty
// string are treated as the same empty key.
//
// It returns the expression NULLIF(col, empty-string-literal) evaluated against
// the source rows. Use it in PartitionBy when a business key such as an invoice
// number may be missing in two representations (NULL or an empty string) and
// those representations must belong to the same partition.
func NullIfEmpty(col string) string {
	return "NULLIF(" + col + ", '')"
}

// CoalesceEmpty normalizes NULL to an empty string so that NULL and the empty
// string are treated as the same empty key.
//
// It returns the expression COALESCE(col, empty-string-literal) evaluated
// against the source rows. Unlike NullIfEmpty, the empty key sorts as an empty
// string rather than NULL, which makes its position in an ascending/descending
// order explicit and independent of the database's NULL ordering.
func CoalesceEmpty(col string) string {
	return "COALESCE(" + col + ", '')"
}
