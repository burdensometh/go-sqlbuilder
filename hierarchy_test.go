// Copyright 2024 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"strings"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleNewHierarchyTreeBuilder() {
	// Walk category tree from roots 1 and 2, at most 5 levels down,
	// then join products to aggregate budget per root.
	htb := NewHierarchyTreeBuilder(
		"cat_tree", "category", "id", "parent_id", 5, 1, 2,
	)

	sb, err := htb.JoinFactTable(
		"product", "p", "p.category_id = cat_tree.id",
		"cat_tree."+htb.RootIDCol(),
		"SUM(p.amount) AS total_amount",
	)
	if err != nil {
		panic(err)
	}
	sb.GroupBy("cat_tree." + htb.RootIDCol())

	sql, args := sb.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// WITH RECURSIVE cat_tree (id, parent_id, root_id, depth, path, is_cycle) AS ((SELECT t.`id`, t.`parent_id`, t.`id` AS `root_id`, 0 AS `depth`, CONCAT('/', `t`.`id`) AS `path`, 0 AS `is_cycle` FROM category AS t WHERE t.`id` IN (?, ?)) UNION ALL (SELECT c.`id`, c.`parent_id`, tree.`root_id`, tree.`depth` + 1 AS `depth`, CONCAT(`tree`.`path`, '/', `c`.`id`) AS `path`, CASE WHEN FIND_IN_SET(`c`.`id`, REPLACE(SUBSTRING(`tree`.`path`, 2), '/', ',')) > 0 THEN 1 ELSE 0 END AS `is_cycle` FROM category AS c JOIN cat_tree AS tree ON c.`parent_id` = tree.`id` WHERE tree.`depth` < 5 AND tree.`is_cycle` = 0)) SELECT cat_tree.root_id, SUM(p.amount) AS total_amount FROM cat_tree LEFT JOIN product AS p ON p.category_id = cat_tree.id WHERE cat_tree.is_cycle = 0 GROUP BY cat_tree.root_id
	// [1 2]
}

func TestHierarchyTreeBuilderMySQL(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder(
		"cat_tree", "category", "id", "parent_id", 3, 1, 2,
	)

	sql, args, err := htb.BuildE()
	a.NilError(err)
	a.Equal(args, []interface{}{1, 2})

	want := "WITH RECURSIVE cat_tree (id, parent_id, root_id, depth, path, is_cycle) AS (" +
		"(SELECT t.`id`, t.`parent_id`, t.`id` AS `root_id`, 0 AS `depth`, CONCAT('/', `t`.`id`) AS `path`, 0 AS `is_cycle` FROM category AS t WHERE t.`id` IN (?, ?)) " +
		"UNION ALL " +
		"(SELECT c.`id`, c.`parent_id`, tree.`root_id`, tree.`depth` + 1 AS `depth`, CONCAT(`tree`.`path`, '/', `c`.`id`) AS `path`, CASE WHEN FIND_IN_SET(`c`.`id`, REPLACE(SUBSTRING(`tree`.`path`, 2), '/', ',')) > 0 THEN 1 ELSE 0 END AS `is_cycle` FROM category AS c JOIN cat_tree AS tree ON c.`parent_id` = tree.`id` WHERE tree.`depth` < 3 AND tree.`is_cycle` = 0))"
	a.Equal(sql, want)

	a.Equal(htb.Cols(), []string{"id", "parent_id", "root_id", "depth", "path", "is_cycle"})
	a.Equal(htb.TableName(), "cat_tree")
}

func TestHierarchyTreeBuilderCycleAndDepth(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder("cat_tree", "category", "id", "parent_id", 3, 1, 2)

	sql, _, err := htb.BuildE()
	a.NilError(err)

	// Fixed roots: anchor filters by bound root ids, no raw interpolation.
	a.Assert(strings.Contains(sql, "t.`id` IN (?, ?)"))
	// Root id is propagated down every row so different roots never mix.
	a.Assert(strings.Contains(sql, "t.`id` AS `root_id`"))
	a.Assert(strings.Contains(sql, "tree.`root_id`"))
	// Anchor starts at depth 0; recursive step increments and is bounded.
	a.Assert(strings.Contains(sql, "0 AS `depth`"))
	a.Assert(strings.Contains(sql, "tree.`depth` + 1 AS `depth`"))
	a.Assert(strings.Contains(sql, "tree.`depth` < 3"))
	// Ancestor path starts at the root id and grows on every step.
	a.Assert(strings.Contains(sql, "CONCAT('/', `t`.`id`) AS `path`"))
	a.Assert(strings.Contains(sql, "CONCAT(`tree`.`path`, '/', `c`.`id`) AS `path`"))
	// Cycle marker is emitted on the row that would close the cycle.
	a.Assert(strings.Contains(sql, "CASE WHEN FIND_IN_SET(`c`.`id`, REPLACE(SUBSTRING(`tree`.`path`, 2), '/', ',')) > 0 THEN 1 ELSE 0 END AS `is_cycle`"))
	// Stable truncation: a flagged row is never expanded again.
	a.Assert(strings.Contains(sql, "tree.`is_cycle` = 0"))
}

func TestHierarchyTreeBuilderFlavors(t *testing.T) {
	a := assert.New(t)

	flavors := []Flavor{MySQL, PostgreSQL, SQLite, SQLServer, Oracle, ClickHouse, Presto, Doris, Informix}
	for _, flavor := range flavors {
		htb := flavor.NewHierarchyTreeBuilder(
			"cat_tree", "category", "id", "parent_id", 4, 10,
		)

		sql, args, err := htb.BuildWithFlavorE(flavor)
		a.NilError(err)
		a.Equal(args, []interface{}{10})
		a.Assert(strings.HasPrefix(sql, "WITH RECURSIVE cat_tree"))
		// Every flavor keeps the depth bound and the cycle truncation guard.
		a.Assert(strings.Contains(sql, flavor.Quote("depth")+" < 4"))
		a.Assert(strings.Contains(sql, "tree."+flavor.Quote("is_cycle")+" = 0"))
		// Every flavor emits a cycle marker column.
		a.Assert(strings.Contains(sql, "AS "+flavor.Quote("is_cycle")))
	}
}

func TestHierarchyTreeBuilderErrors(t *testing.T) {
	a := assert.New(t)

	// No root.
	htb := NewHierarchyTreeBuilder("t", "category", "id", "parent_id", 3)
	_, _, err := htb.BuildE()
	a.Equal(err, ErrHierarchyNoRoot)

	// Negative depth.
	htb = NewHierarchyTreeBuilder("t", "category", "id", "parent_id", -1, 1)
	_, _, err = htb.BuildE()
	a.Equal(err, ErrHierarchyInvalidMaxDepth)

	// Default depth kicks in for 0.
	htb = NewHierarchyTreeBuilder("t", "category", "id", "parent_id", 0, 1)
	sql, _, err := htb.BuildE()
	a.NilError(err)
	a.Assert(strings.Contains(sql, MySQL.Quote("depth")+" < 32"))

	// Build panics on misconfiguration, like other builders do on misuse.
	func() {
		defer func() {
			a.Assert(recover() != nil)
		}()
		NewHierarchyTreeBuilder("t", "category", "id", "parent_id", 3).Build()
	}()
}

func TestHierarchyTreeBuilderIdempotent(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder(
		"cat_tree", "category", "id", "parent_id", 3, 1, 2,
	)

	s1, a1, err := htb.BuildE()
	a.NilError(err)
	s2, a2, err := htb.BuildE()
	a.NilError(err)
	a.Equal(s1, s2)
	a.Equal(a1, a2)

	// String() and Build() agree.
	a.Equal(htb.String(), s1)
}

func TestHierarchyTreeBuilderCustomCols(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder("tree", "cat", "node_id", "pid", 2, 7).
		SetRootIDCol("root_node").
		SetDepthCol("lvl").
		SetPathCol("ancestry").
		SetCycleCol("cycled")

	a.Equal(htb.Cols(), []string{"node_id", "pid", "root_node", "lvl", "ancestry", "cycled"})
	a.Equal(htb.RootIDCol(), "root_node")
	a.Equal(htb.DepthCol(), "lvl")
	a.Equal(htb.PathCol(), "ancestry")
	a.Equal(htb.CycleCol(), "cycled")

	sql, args, err := htb.BuildE()
	a.NilError(err)
	a.Equal(args, []interface{}{7})
	a.Assert(strings.Contains(sql, "root_node"))
	a.Assert(strings.Contains(sql, "lvl"))
	a.Assert(strings.Contains(sql, "ancestry"))
	a.Assert(strings.Contains(sql, "cycled"))
}

func TestHierarchyTreeBuilderRootsAndFlavor(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 2, 1).
		Roots(100, 200)

	sql, args, err := htb.BuildE()
	a.NilError(err)
	a.Equal(args, []interface{}{100, 200})
	a.Assert(strings.Contains(sql, "IN (?, ?)"))

	// Flavor switch changes identifier quoting and placeholder style.
	old := htb.SetFlavor(PostgreSQL)
	a.Equal(old, MySQL)
	a.Equal(htb.Flavor(), PostgreSQL)
	pgSQL, pgArgs, err := htb.BuildE()
	a.NilError(err)
	a.Equal(pgArgs, []interface{}{100, 200})
	a.Assert(strings.Contains(pgSQL, `t."id" IN ($1, $2)`))
}

func TestHierarchyTreeBuilderJoinFactTable(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder(
		"cat_tree", "category", "id", "parent_id", 5, 1, 2,
	)

	sb, err := htb.JoinFactTable(
		"product", "p", "p.category_id = cat_tree.id",
		"cat_tree."+htb.RootIDCol(),
		"SUM(p.amount) AS total_amount",
	)
	a.NilError(err)
	sb.GroupBy("cat_tree." + htb.RootIDCol())

	sql, args := sb.Build()
	a.Equal(args, []interface{}{1, 2})
	// Tree drives FROM, fact table is LEFT JOINed exactly once; flagged
	// cycle rows (duplicated node ids) are filtered to avoid double count.
	a.Assert(strings.Contains(sql, "FROM cat_tree LEFT JOIN product AS p ON p.category_id = cat_tree.id"))
	a.Assert(strings.Contains(sql, "WHERE cat_tree.is_cycle = 0"))
	a.Assert(strings.Contains(sql, "GROUP BY cat_tree.root_id"))
}

func TestHierarchyTreeBuilderSetMaxDepth(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 9, 1)
	htb.SetMaxDepth(2)

	sql, _, err := htb.BuildE()
	a.NilError(err)
	a.Assert(strings.Contains(sql, MySQL.Quote("depth")+" < 2"))
}

func TestHierarchyTreeBuilderCTEQueryAndBuilder(t *testing.T) {
	a := assert.New(t)
	htb := NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 3, 1)

	// Standalone CTE query can be embedded in any WITH RECURSIVE clause.
	cteQuery, err := htb.CTEQuery()
	a.NilError(err)
	a.Equal(cteQuery.TableName(), "tree")

	// Wrapped CTE builder carries the RECURSIVE keyword.
	cteBuilder, err := htb.CTEBuilder()
	a.NilError(err)
	a.Assert(strings.HasPrefix(cteBuilder.String(), "WITH RECURSIVE tree"))

	// Both helpers surface configuration errors.
	bad := NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 3)
	_, err = bad.CTEQuery()
	a.Equal(err, ErrHierarchyNoRoot)
	_, err = bad.CTEBuilder()
	a.Equal(err, ErrHierarchyNoRoot)

	// JoinFactTable propagates the error too.
	_, err = bad.JoinFactTable("p", "p", "p.id = tree.id")
	a.Equal(err, ErrHierarchyNoRoot)
}

func TestHierarchyTreeBuilderFlavorSnapshots(t *testing.T) {
	a := assert.New(t)

	// PostgreSQL uses || and ::TEXT with an unnest-based cycle test.
	pg := PostgreSQL.NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 2, 9)
	sql, _, err := pg.BuildWithFlavorE(PostgreSQL)
	a.NilError(err)
	a.Assert(strings.Contains(sql, `'/' || "t"."id"::TEXT AS "path"`))
	a.Assert(strings.Contains(sql, `unnest(string_to_array(substr("tree"."path", 2), '/'))`))

	// SQLite uses || without casts and an INSTR-based cycle test.
	sq := SQLite.NewHierarchyTreeBuilder("tree", "cat", "id", "parent_id", 2, 9)
	sql, _, err = sq.BuildWithFlavorE(SQLite)
	a.NilError(err)
	a.Assert(strings.Contains(sql, `'/' || "t"."id" AS "path"`))
	a.Assert(strings.Contains(sql, "INSTR("))
}
