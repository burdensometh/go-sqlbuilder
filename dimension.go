// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

// DimensionJoin describes a point-in-time ("as of") join between a fact
// table and a versioned dimension table.
//
// A dimension version is valid over the left-closed, right-open interval
// [StartTime, EndTime). For a fact row the generated join predicate is
//
//	FactTime >= StartTime AND (EndTime IS NULL OR FactTime < EndTime)
//
// so a row whose EndTime is NULL is treated as valid forever.
//
// The default join is a LEFT JOIN. Facts that have no matching version are
// therefore kept instead of being silently dropped. Set Option explicitly to
// another join type when that is not desired.
//
// Overlapping dimension versions are deliberately not collapsed by this
// builder. When two or more versions cover the same fact time, the join
// produces one row per matching version, which lets a data warehouse caller
// find the conflicting facts, e.g.
//
//	sb := NewSelectBuilder()
//	sb.Select("f.id", "c.category_id")
//	sb.From("txn f")
//	sb.JoinDimension(&DimensionJoin{
//		Table:        "product_category c",
//		FactKey:      "f.product_id",
//		DimensionKey: "c.product_id",
//		FactTime:     "f.txn_time",
//		StartTime:    "c.start_time",
//		EndTime:      "c.end_time",
//	})
//	sb.GroupBy("f.id").Having("COUNT(*) > 1")
//
// Such a query returns the facts whose product id maps to more than one
// version at the transaction time.
type DimensionJoin struct {
	// Table is the dimension table, optionally with an alias, e.g.
	// "product_category c".
	// It is required; a DimensionJoin with an empty Table is ignored.
	Table string

	// FactKey and DimensionKey build the key condition
	// "FactKey = DimensionKey".
	// If either of them is empty, the key condition is omitted. Composite
	// keys can be added through On instead.
	FactKey      string
	DimensionKey string

	// FactTime is the fact table timestamp used to look up the matching
	// version, e.g. "f.txn_time".
	// If it is empty, no time range condition is generated.
	FactTime string

	// StartTime is the dimension column holding the inclusive start of the
	// validity interval, e.g. "c.start_time".
	// It is ignored when FactTime is empty.
	StartTime string

	// EndTime is the dimension column holding the exclusive end of the
	// validity interval, e.g. "c.end_time".
	// A NULL value means the version is still valid. Leave EndTime empty if
	// the dimension table does not have an end column; only the lower bound
	// is generated in that case.
	// It is ignored when FactTime is empty.
	EndTime string

	// Option is the join type. The zero value means LeftJoin so that facts
	// without a matching version are retained. Set InnerJoin explicitly to
	// drop them.
	Option JoinOption

	// On contains extra join conditions appended after the point-in-time
	// range predicate, e.g. a soft-delete flag.
	On []string
}

// NewDimensionJoin creates a DimensionJoin for a dimension table.
//
// The table may carry an alias, e.g. "product_category c".
func NewDimensionJoin(table string) *DimensionJoin {
	return &DimensionJoin{
		Table: table,
	}
}

// OnExpressions returns the join conditions generated from the dimension
// join, in the same order as they appear in the JOIN clause.
//
// It returns nil if the DimensionJoin is nil or has no table.
func (dj *DimensionJoin) OnExpressions() []string {
	if dj == nil || dj.Table == "" {
		return nil
	}

	exprs := make([]string, 0, 3+len(dj.On))

	if dj.FactKey != "" && dj.DimensionKey != "" {
		exprs = append(exprs, dj.FactKey+" = "+dj.DimensionKey)
	}

	if dj.FactTime != "" {
		if dj.StartTime != "" {
			exprs = append(exprs, dj.FactTime+" >= "+dj.StartTime)
		}

		if dj.EndTime != "" {
			exprs = append(exprs, "("+dj.EndTime+" IS NULL OR "+dj.FactTime+" < "+dj.EndTime+")")
		}
	}

	return append(exprs, filterEmptyStrings(dj.On)...)
}

func (dj *DimensionJoin) joinOption() JoinOption {
	if dj.Option != "" {
		return dj.Option
	}

	return LeftJoin
}

// JoinDimension adds a point-in-time join between the facts in SELECT and a
// versioned dimension table.
//
// The dimension version is matched with the left-closed, right-open time
// interval [StartTime, EndTime). A NULL EndTime means the version is still
// valid. The join is a LEFT JOIN by default, so facts without any matching
// version are preserved.
//
// Overlapping dimension versions are not merged; a fact matching several
// versions produces several rows, which allows callers to detect conflicting
// mappings.
//
// A nil DimensionJoin or one without a table is ignored.
func (sb *SelectBuilder) JoinDimension(dimension *DimensionJoin) *SelectBuilder {
	if dimension == nil || dimension.Table == "" {
		return sb
	}

	return sb.JoinWithOption(dimension.joinOption(), dimension.Table, dimension.OnExpressions()...)
}

// JoinPointInTime is a shorthand for JoinDimension with a left join.
//
// It joins table on factKey = dimensionKey and matches factTime against the
// left-closed, right-open interval [startTime, endTime). An empty endTime
// omits the upper bound condition. Extra onExpr are appended to the join
// conditions.
//
// See DimensionJoin for details, including how to detect dimension versions
// that overlap.
func (sb *SelectBuilder) JoinPointInTime(table, factKey, dimensionKey, factTime, startTime, endTime string, onExpr ...string) *SelectBuilder {
	return sb.JoinDimension(&DimensionJoin{
		Table:        table,
		FactKey:      factKey,
		DimensionKey: dimensionKey,
		FactTime:     factTime,
		StartTime:    startTime,
		EndTime:      endTime,
		On:           onExpr,
	})
}
