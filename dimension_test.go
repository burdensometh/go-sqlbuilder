// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleSelectBuilder_JoinDimension() {
	// Transaction facts only store the product id. The product category
	// dimension keeps one row per version with [start_time, end_time).
	sb := NewSelectBuilder()
	sb.Select("f.id", "c.category_id")
	sb.From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
		EndTime:      "c.end_time",
	})

	sql, args := sb.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT f.id, c.category_id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time)
	// []
}

func ExampleSelectBuilder_JoinDimension_conflicts() {
	// Overlapping dimension versions are not merged. A fact that matches
	// more than one version shows up once per version, so grouping by fact
	// and counting lets a warehouse caller find the conflicting facts.
	sb := NewSelectBuilder()
	sb.Select("f.id", "COUNT(*) AS version_count")
	sb.From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
		EndTime:      "c.end_time",
	})
	sb.GroupBy("f.id").Having("COUNT(*) > 1")

	sql, args := sb.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT f.id, COUNT(*) AS version_count FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time) GROUP BY f.id HAVING COUNT(*) > 1
	// []
}

func ExampleSelectBuilder_JoinPointInTime() {
	sb := NewSelectBuilder()
	sb.Select("f.id", "c.category_id")
	sb.From("txn f")
	sb.JoinPointInTime(
		"product_category c",
		"f.product_id", "c.product_id",
		"f.txn_time",
		"c.start_time", "c.end_time",
		"c.deleted = 0",
	)

	sql, args := sb.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT f.id, c.category_id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time) AND c.deleted = 0
	// []
}

func TestJoinDimension(t *testing.T) {
	a := assert.New(t)

	// The default join keeps facts without any matching version.
	sb := NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
		EndTime:      "c.end_time",
	})
	sql, args := sb.Build()
	a.Equal("SELECT f.id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time)", sql)
	a.Equal([]interface{}(nil), args)

	// An explicit option overrides the default left join.
	sb = NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
		EndTime:      "c.end_time",
		Option:       InnerJoin,
	})
	a.Equal("SELECT f.id FROM txn f INNER JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time)", sb.String())

	// A dimension without an end column only has the inclusive lower bound.
	sb = NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
	})
	a.Equal("SELECT f.id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time", sb.String())

	// Composite keys and extra conditions can be supplied via On.
	sb = NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:     "product_category c",
		FactTime:  "f.txn_time",
		StartTime: "c.start_time",
		EndTime:   "c.end_time",
		On: []string{
			"f.tenant_id = c.tenant_id",
			"",
			"c.deleted = 0",
		},
	})
	a.Equal("SELECT f.id FROM txn f LEFT JOIN product_category c ON f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time) AND f.tenant_id = c.tenant_id AND c.deleted = 0", sb.String())

	// A generated key is omitted when the fact time is unknown.
	sb = NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
	})
	a.Equal("SELECT f.id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id", sb.String())

	// Nil and empty dimensions are ignored.
	sb = NewSelectBuilder()
	sb.Select("f.id").From("txn f")
	sb.JoinDimension(nil)
	sb.JoinDimension(&DimensionJoin{})
	a.Equal("SELECT f.id FROM txn f", sb.String())
}

func TestJoinDimensionAppendsToExistingJoins(t *testing.T) {
	a := assert.New(t)

	sb := NewSelectBuilder()
	sb.Select("f.id", "s.name", "c.category_id").From("txn f")
	sb.Join("shop s", "f.shop_id = s.id")
	sb.JoinDimension(&DimensionJoin{
		Table:        "product_category c",
		FactKey:      "f.product_id",
		DimensionKey: "c.product_id",
		FactTime:     "f.txn_time",
		StartTime:    "c.start_time",
		EndTime:      "c.end_time",
	})

	a.Equal("SELECT f.id, s.name, c.category_id FROM txn f JOIN shop s ON f.shop_id = s.id LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time)", sb.String())
}

func TestJoinDimensionFlavors(t *testing.T) {
	a := assert.New(t)

	newBuilder := func(flavor Flavor) *SelectBuilder {
		sb := flavor.NewSelectBuilder()
		sb.Select("f.id").From("txn f")
		sb.JoinDimension(&DimensionJoin{
			Table:        "product_category c",
			FactKey:      "f.product_id",
			DimensionKey: "c.product_id",
			FactTime:     "f.txn_time",
			StartTime:    "c.start_time",
			EndTime:      "c.end_time",
		})
		return sb
	}

	for _, flavor := range []Flavor{MySQL, PostgreSQL, SQLite, SQLServer, ClickHouse, Doris, Presto} {
		sql, _ := newBuilder(flavor).Build()
		a.Equal("SELECT f.id FROM txn f LEFT JOIN product_category c ON f.product_id = c.product_id AND f.txn_time >= c.start_time AND (c.end_time IS NULL OR f.txn_time < c.end_time)", sql)

		// The point-in-time predicate is database neutral; interpolation must
		// not touch generated column names or arguments.
		interpolated, err := flavor.Interpolate(sql, nil)
		a.NilError(err)
		a.Equal(sql, interpolated)
	}
}
