// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleUnionAllWithSchema() {
	// Target schema: the new source adds a nullable column "region".
	cols := []UnionColumn{
		{Name: "id"},
		{Name: "amount"},
		{Name: "region", Type: "VARCHAR(32)", Optional: true},
	}

	// Legacy source stores amount in cents and has no region column.
	legacy := NewSelectBuilder()
	legacy.Select("order_id", "amount_cent").From("legacy_orders")

	// New source has the columns in a different order.
	modern := NewSelectBuilder()
	modern.Select("region", "amount_yuan", "id").From("modern_orders")

	ub, err := UnionAllWithSchema(cols, []UnionSource{
		{
			Name:    "legacy_orders",
			Builder: legacy,
			Columns: map[string]string{
				"id":     "src.order_id",
				"amount": "src.amount_cent / 100",
			},
		},
		{
			Name:    "modern_orders",
			Builder: modern,
			Columns: map[string]string{
				"id":     "src.id",
				"amount": "src.amount_yuan",
				"region": "src.region",
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Embed the aligned UNION ALL into an aggregation by business region.
	report := NewSelectBuilder()
	report.Select("region", "COALESCE(SUM(amount), 0) AS total")
	report.From(report.BuilderAs(ub, "orders"))
	report.GroupBy("region")

	sql, args := report.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT region, COALESCE(SUM(amount), 0) AS total FROM ((SELECT src.order_id AS id, src.amount_cent / 100 AS amount, CAST(NULL AS VARCHAR(32)) AS region FROM (SELECT order_id, amount_cent FROM legacy_orders) AS src) UNION ALL (SELECT src.id AS id, src.amount_yuan AS amount, src.region AS region FROM (SELECT region, amount_yuan, id FROM modern_orders) AS src)) AS orders GROUP BY region
	// []
}

func TestUnionAllWithSchemaReordersAndFillsNulls(t *testing.T) {
	a := assert.New(t)

	cols := []UnionColumn{
		{Name: "id"},
		{Name: "name"},
		{Name: "score", Type: "DOUBLE", Optional: true},
	}

	// Old source: columns in reverse order, no score.
	oldSource := Select("nick_name", "uid").From("old_users")

	// New source: all columns, yet in a different order.
	newSource := Select("score", "id", "name").From("new_users")

	ub, err := UnionAllWithSchema(cols, []UnionSource{
		{
			Name:    "old",
			Builder: oldSource,
			Columns: map[string]string{
				"id":   "src.uid",
				"name": "src.nick_name",
			},
		},
		{
			Name:    "new",
			Builder: newSource,
			Columns: map[string]string{
				"name":  "src.name",
				"id":    "src.id",
				"score": "src.score",
			},
		},
	})
	a.NilError(err)

	sql, args := ub.Build()
	a.Equal(sql,
		"(SELECT src.uid AS id, src.nick_name AS name, CAST(NULL AS DOUBLE) AS score FROM (SELECT nick_name, uid FROM old_users) AS src) "+
			"UNION ALL "+
			"(SELECT src.id AS id, src.name AS name, src.score AS score FROM (SELECT score, id, name FROM new_users) AS src)")
	a.Assert(len(args) == 0)
}

func TestUnionAllWithSchemaMissingRequiredColumnReportsSourceAndColumn(t *testing.T) {
	a := assert.New(t)

	cols := []UnionColumn{
		{Name: "id"},
		{Name: "region", Type: "VARCHAR(16)", Optional: true},
		{Name: "amount"},
	}

	source := Select("region").From("sales")

	_, err := UnionAllWithSchema(cols, []UnionSource{
		{
			Name:    "sales",
			Builder: source,
			Columns: map[string]string{
				"region": "src.region",
			},
		},
	})

	a.Assert(err != nil)
	a.Assert(errors.Is(err, ErrUnionMissingRequiredColumn))
	a.Assert(strings.Contains(err.Error(), "source sales"))
	a.Assert(strings.Contains(err.Error(), "column id"))
}

func TestUnionAllWithSchemaValidationErrors(t *testing.T) {
	a := assert.New(t)

	sb := Select("id").From("t")
	validCols := []UnionColumn{{Name: "id"}}
	validSources := []UnionSource{{Name: "s", Builder: sb, Columns: map[string]string{"id": "src.id"}}}

	// No target columns.
	_, err := UnionAllWithSchema(nil, validSources)
	a.Assert(errors.Is(err, ErrUnionNoTargetColumn))

	// No sources.
	_, err = UnionAllWithSchema(validCols, nil)
	a.Assert(errors.Is(err, ErrUnionNoSource))

	// Empty target column name.
	_, err = UnionAllWithSchema([]UnionColumn{{Name: ""}}, validSources)
	a.Assert(errors.Is(err, ErrUnionEmptyName))

	// Optional column without a type cannot produce a typed NULL.
	_, err = UnionAllWithSchema(
		[]UnionColumn{{Name: "id"}, {Name: "note", Optional: true}},
		validSources,
	)
	a.Assert(errors.Is(err, ErrUnionOptionalColumnNoType))
	a.Assert(strings.Contains(err.Error(), "note"))

	// Duplicate target columns.
	_, err = UnionAllWithSchema(
		[]UnionColumn{{Name: "id"}, {Name: "id"}},
		validSources,
	)
	a.Assert(errors.Is(err, ErrUnionDuplicateColumn))

	// Source maps an unknown column.
	_, err = UnionAllWithSchema(validCols, []UnionSource{{
		Name:    "s",
		Builder: sb,
		Columns: map[string]string{"id": "src.id", "ghost": "src.ghost"},
	}})
	a.Assert(errors.Is(err, ErrUnionUnknownColumn))
	a.Assert(strings.Contains(err.Error(), "source s"))
	a.Assert(strings.Contains(err.Error(), "column ghost"))

	// Nil builder.
	_, err = UnionAllWithSchema(validCols, []UnionSource{{
		Name:    "broken",
		Builder: nil,
		Columns: map[string]string{"id": "src.id"},
	}})
	a.Assert(errors.Is(err, ErrUnionNilBuilder))
	a.Assert(strings.Contains(err.Error(), "source broken"))

	// Empty source name.
	_, err = UnionAllWithSchema(validCols, []UnionSource{{
		Name:    "",
		Builder: sb,
		Columns: map[string]string{"id": "src.id"},
	}})
	a.Assert(errors.Is(err, ErrUnionEmptyName))
}

func TestUnionAllWithSchemaFlavorAndArgs(t *testing.T) {
	a := assert.New(t)

	cols := []UnionColumn{
		{Name: "id"},
		{Name: "note", Type: "TEXT", Optional: true},
	}

	oldSource := PostgreSQL.NewSelectBuilder()
	oldSource.Select("uid").From("old_t").Where(oldSource.GreaterThan("uid", 10))

	newSource := PostgreSQL.NewSelectBuilder()
	newSource.Select("id", "note").From("new_t").Where(newSource.In("id", 1, 2))

	ub := PostgreSQL.NewUnionBuilder()
	err := ub.UnionAllWithSchema(cols, []UnionSource{
		{Name: "old", Builder: oldSource, Columns: map[string]string{"id": "src.uid"}},
		{Name: "new", Builder: newSource, Columns: map[string]string{"id": "src.id", "note": "src.note"}},
	})
	a.NilError(err)

	sql, args := ub.Build()
	a.Equal(sql,
		"(SELECT src.uid AS id, CAST(NULL AS TEXT) AS note FROM (SELECT uid FROM old_t WHERE uid > $1) AS src) "+
			"UNION ALL "+
			"(SELECT src.id AS id, src.note AS note FROM (SELECT id, note FROM new_t WHERE id IN ($2, $3)) AS src)")
	a.Equal(args, []interface{}{10, 1, 2})
}

func TestUnionAllWithSchemaOnExistingBuilder(t *testing.T) {
	a := assert.New(t)

	ub := NewUnionBuilder()
	sb1 := Select("a").From("t1")
	sb2 := Select("b").From("t2")

	err := ub.UnionAllWithSchema(
		[]UnionColumn{{Name: "v"}},
		[]UnionSource{
			{Name: "t1", Builder: sb1, Columns: map[string]string{"v": "src.a"}},
			{Name: "t2", Builder: sb2, Columns: map[string]string{"v": "src.b"}},
		},
	)
	a.NilError(err)

	ub.OrderByDesc("v").Limit(5)

	sql, _ := ub.Build()
	a.Equal(sql,
		"(SELECT src.a AS v FROM (SELECT a FROM t1) AS src) "+
			"UNION ALL "+
			"(SELECT src.b AS v FROM (SELECT b FROM t2) AS src) "+
			"ORDER BY v DESC LIMIT ?")
}

func TestUnionAllWithSchemaClone(t *testing.T) {
	a := assert.New(t)

	cols := []UnionColumn{{Name: "id"}, {Name: "note", Type: "TEXT", Optional: true}}
	source := Select("uid").From("t")

	ub, err := UnionAllWithSchema(cols, []UnionSource{
		{Name: "t", Builder: source, Columns: map[string]string{"id": "src.uid"}},
	})
	a.NilError(err)

	clone := ub.Clone()
	clone.Limit(1)

	s1, _ := ub.Build()
	s2, args2 := clone.Build()
	a.Assert(s1 != s2)
	a.Assert(strings.HasPrefix(s2, s1))
	a.Assert(strings.Contains(s2, "LIMIT ?"))
	a.Equal(args2, []interface{}{1})
}
