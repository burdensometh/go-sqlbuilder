// Copyright 2024 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
)

// Default column names of the recursive CTE built by HierarchyTreeBuilder.
const (
	defaultHierarchyRootIDCol = "root_id"
	defaultHierarchyDepthCol  = "depth"
	defaultHierarchyPathCol   = "path"
	defaultHierarchyCycleCol  = "is_cycle"

	// defaultHierarchyMaxDepth bounds a walk when the caller passes 0.
	defaultHierarchyMaxDepth = 32
)

// ErrHierarchyNoRoot means no root id is provided to a HierarchyTreeBuilder.
var ErrHierarchyNoRoot = errors.New("go-sqlbuilder: hierarchy tree requires at least one root id")

// ErrHierarchyInvalidMaxDepth means max depth is negative.
var ErrHierarchyInvalidMaxDepth = errors.New("go-sqlbuilder: max depth of hierarchy tree must be greater than or equal to 0")

// HierarchyTreeBuilder builds a recursive CTE which walks a parent/child
// category table from a fixed set of roots down to a bounded depth.
//
// The generated CTE always carries the root id alongside every row, so rows
// descended from different roots never mix when the CTE is joined to a fact
// table. An ancestor path and a cycle flag are maintained during the walk.
// When the input table accidentally contains a cycle, the row whose edge
// would lead back to an already-visited ancestor is still emitted with
// is_cycle = 1 (the marker) but is never expanded further (the stable
// truncation), so recursion terminates instead of looping forever.
type HierarchyTreeBuilder struct {
	flavor Flavor

	cteName string
	table   string

	idCol     string
	parentCol string
	rootIDCol string
	depthCol  string
	pathCol   string
	cycleCol  string

	rootIDs  []interface{}
	maxDepth int
}

var _ Builder = (*HierarchyTreeBuilder)(nil)

// NewHierarchyTreeBuilder creates a new HierarchyTreeBuilder with default flavor.
//
// table is the category table which stores parent/child relations in idCol
// and parentCol. roots are the fixed ids the walk starts from; at least one
// root is required. maxDepth bounds the number of levels below a root
// (the root itself sits at depth 0); pass 0 to use the library default.
func NewHierarchyTreeBuilder(cteName, table, idCol, parentCol string, maxDepth int, roots ...interface{}) *HierarchyTreeBuilder {
	return DefaultFlavor.NewHierarchyTreeBuilder(cteName, table, idCol, parentCol, maxDepth, roots...)
}

// NewHierarchyTreeBuilder creates a new HierarchyTreeBuilder with flavor.
func (f Flavor) NewHierarchyTreeBuilder(cteName, table, idCol, parentCol string, maxDepth int, roots ...interface{}) *HierarchyTreeBuilder {
	return &HierarchyTreeBuilder{
		flavor:    f,
		cteName:   cteName,
		table:     table,
		idCol:     idCol,
		parentCol: parentCol,
		rootIDCol: defaultHierarchyRootIDCol,
		depthCol:  defaultHierarchyDepthCol,
		pathCol:   defaultHierarchyPathCol,
		cycleCol:  defaultHierarchyCycleCol,
		rootIDs:   append([]interface{}(nil), roots...),
		maxDepth:  maxDepth,
	}
}

// SetRootIDCol overrides the output column name of the root id.
func (htb *HierarchyTreeBuilder) SetRootIDCol(col string) *HierarchyTreeBuilder {
	htb.rootIDCol = col
	return htb
}

// SetDepthCol overrides the output column name of the depth.
func (htb *HierarchyTreeBuilder) SetDepthCol(col string) *HierarchyTreeBuilder {
	htb.depthCol = col
	return htb
}

// SetPathCol overrides the output column name of the ancestor path.
func (htb *HierarchyTreeBuilder) SetPathCol(col string) *HierarchyTreeBuilder {
	htb.pathCol = col
	return htb
}

// SetCycleCol overrides the output column name of the cycle flag.
func (htb *HierarchyTreeBuilder) SetCycleCol(col string) *HierarchyTreeBuilder {
	htb.cycleCol = col
	return htb
}

// Roots replaces the fixed root ids of the walk.
func (htb *HierarchyTreeBuilder) Roots(roots ...interface{}) *HierarchyTreeBuilder {
	htb.rootIDs = append([]interface{}(nil), roots...)
	return htb
}

// SetMaxDepth bounds the number of levels below a root.
// The root itself sits at depth 0, so maxDepth 1 yields the root and its
// direct children only. Pass 0 to restore the library default.
func (htb *HierarchyTreeBuilder) SetMaxDepth(maxDepth int) *HierarchyTreeBuilder {
	htb.maxDepth = maxDepth
	return htb
}

// TableName returns the recursive CTE table name.
func (htb *HierarchyTreeBuilder) TableName() string {
	return htb.cteName
}

// Cols returns all output columns of the recursive CTE in order.
func (htb *HierarchyTreeBuilder) Cols() []string {
	return []string{htb.idCol, htb.parentCol, htb.rootIDCol, htb.depthCol, htb.pathCol, htb.cycleCol}
}

// RootIDCol returns the output column name of the root id.
func (htb *HierarchyTreeBuilder) RootIDCol() string {
	return htb.rootIDCol
}

// DepthCol returns the output column name of the depth.
func (htb *HierarchyTreeBuilder) DepthCol() string {
	return htb.depthCol
}

// PathCol returns the output column name of the ancestor path.
func (htb *HierarchyTreeBuilder) PathCol() string {
	return htb.pathCol
}

// CycleCol returns the output column name of the cycle flag.
func (htb *HierarchyTreeBuilder) CycleCol() string {
	return htb.cycleCol
}

// Flavor returns flavor of builder.
func (htb *HierarchyTreeBuilder) Flavor() Flavor {
	return htb.flavor
}

// SetFlavor sets the flavor of compiled sql.
func (htb *HierarchyTreeBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = htb.flavor
	htb.flavor = flavor
	return
}

func (htb *HierarchyTreeBuilder) effectiveMaxDepth() (int, error) {
	if htb.maxDepth < 0 {
		return 0, ErrHierarchyInvalidMaxDepth
	}

	if htb.maxDepth == 0 {
		return defaultHierarchyMaxDepth, nil
	}

	return htb.maxDepth, nil
}

// anchorPathExpr renders the path of a root node, i.e. "/" + its own id.
func (htb *HierarchyTreeBuilder) anchorPathExpr(flavor Flavor, tableAlias string) string {
	q := flavor.Quote
	id := q(tableAlias) + "." + q(htb.idCol)

	switch flavor {
	case MySQL, Doris, ClickHouse:
		return fmt.Sprintf("CONCAT('/', %s) AS %s", id, q(htb.pathCol))

	case SQLite:
		// || coerces integers to text in SQLite.
		return fmt.Sprintf("'/' || %s AS %s", id, q(htb.pathCol))

	case PostgreSQL, Presto, Informix:
		return fmt.Sprintf("'/' || %s::TEXT AS %s", id, q(htb.pathCol))

	case SQLServer:
		return fmt.Sprintf("'/' + CAST(%s AS NVARCHAR(MAX)) AS %s", id, q(htb.pathCol))

	case Oracle:
		return fmt.Sprintf("'/' || TO_CHAR(%s) AS %s", id, q(htb.pathCol))

	default:
		return fmt.Sprintf("CONCAT('/', %s) AS %s", id, q(htb.pathCol))
	}
}

// pathAppendExpr appends the child id to the ancestor path carried by a row
// of the recursive CTE. The path is a stringified id list separated by "/",
// always starting with "/", e.g. "/1/4/9".
func (htb *HierarchyTreeBuilder) pathAppendExpr(flavor Flavor, cteAlias string) string {
	q := flavor.Quote
	childID := q("c") + "." + q(htb.idCol)
	path := q(cteAlias) + "." + q(htb.pathCol)

	switch flavor {
	case MySQL, Doris, ClickHouse:
		// CONCAT implicitly casts the id to string.
		return fmt.Sprintf("CONCAT(%s, '/', %s)", path, childID)

	case SQLite:
		return fmt.Sprintf("%s || '/' || %s", path, childID)

	case PostgreSQL, Presto, Informix:
		return fmt.Sprintf("%s || '/' || %s::TEXT", path, childID)

	case SQLServer:
		return fmt.Sprintf("%s + '/' + CAST(%s AS NVARCHAR(MAX))", path, childID)

	case Oracle:
		return fmt.Sprintf("%s || '/' || TO_CHAR(%s)", path, childID)

	default:
		return fmt.Sprintf("CONCAT(%s, '/', %s)", path, childID)
	}
}

// pathContainsExpr checks whether the child id already appears on the
// ancestor path, i.e. walking into it would close a cycle.
func (htb *HierarchyTreeBuilder) pathContainsExpr(flavor Flavor, cteAlias string) string {
	q := flavor.Quote
	childID := q("c") + "." + q(htb.idCol)
	path := q(cteAlias) + "." + q(htb.pathCol)

	switch flavor {
	case MySQL, Doris:
		// FIND_IN_SET searches a comma-separated list, so compare against
		// a comma-separated rendering of the path. It casts its first
		// argument to a string implicitly.
		return fmt.Sprintf("FIND_IN_SET(%s, REPLACE(SUBSTRING(%s, 2), '/', ',')) > 0", childID, path)

	case ClickHouse:
		return fmt.Sprintf("has(splitByChar('/', substring(%s, 2)), toString(%s))", path, childID)

	case SQLite:
		return fmt.Sprintf("INSTR(',' || REPLACE(SUBSTR(%s, 2), '/', ',') || ',', ',' || %s || ',') > 0", path, childID)

	case PostgreSQL, Presto, Informix:
		return fmt.Sprintf("(SELECT COUNT(1) FROM unnest(string_to_array(substr(%s, 2), '/')) AS p(id) WHERE p.id = %s::TEXT) > 0", path, childID)

	case SQLServer:
		return fmt.Sprintf("CHARINDEX(',' + CAST(%s AS NVARCHAR(MAX)) + ',', ',' + REPLACE(SUBSTRING(%s, 2, LEN(%s)), '/', ',') + ',') > 0", childID, path, path)

	case Oracle:
		return fmt.Sprintf("INSTR(',' || REPLACE(SUBSTR(%s, 2), '/', ',') || ',', ',' || TO_CHAR(%s) || ',') > 0", path, childID)

	default:
		return fmt.Sprintf("FIND_IN_SET(%s, REPLACE(SUBSTRING(%s, 2), '/', ',')) > 0", childID, path)
	}
}

// buildCTEQuery assembles the anchor and recursive members into the CTE
// query backing the tree. Every builder owns fresh internal Args, so
// calling Build repeatedly never accumulates placeholders; args bubble up
// through the nested builders when the CTE is compiled.
func (htb *HierarchyTreeBuilder) buildCTEQuery(flavor Flavor) (*CTEQueryBuilder, error) {
	if len(htb.rootIDs) == 0 {
		return nil, ErrHierarchyNoRoot
	}

	maxDepth, err := htb.effectiveMaxDepth()
	if err != nil {
		return nil, err
	}

	q := flavor.Quote
	cteAlias := "tree"

	// Anchor member: every requested root starts its own group at depth 0.
	// Root ids are bound as args via Cond.In so the value is never
	// interpolated raw.
	anchor := flavor.NewSelectBuilder().Select(
		"t."+q(htb.idCol),
		"t."+q(htb.parentCol),
		"t."+q(htb.idCol)+" AS "+q(htb.rootIDCol),
		"0 AS "+q(htb.depthCol),
		htb.anchorPathExpr(flavor, "t"),
		"0 AS "+q(htb.cycleCol),
	).From(htb.table + " AS t")
	anchor.Where(anchor.In("t."+q(htb.idCol), htb.rootIDs...))

	cycleTest := htb.pathContainsExpr(flavor, cteAlias)

	// Recursive member: join the next level onto the working set and stop
	// expanding at the depth bound. A row whose edge would close a cycle
	// is emitted with is_cycle = 1 as the marker, but the is_cycle = 0
	// predicate below makes sure such a row is never expanded again: this
	// is the stable truncation that keeps recursion finite on cyclic input.
	recursive := flavor.NewSelectBuilder().Select(
		"c."+q(htb.idCol),
		"c."+q(htb.parentCol),
		"tree."+q(htb.rootIDCol),
		fmt.Sprintf("tree.%s + 1 AS %s", q(htb.depthCol), q(htb.depthCol)),
		htb.pathAppendExpr(flavor, cteAlias)+" AS "+q(htb.pathCol),
		fmt.Sprintf("CASE WHEN %s THEN 1 ELSE 0 END AS %s", cycleTest, q(htb.cycleCol)),
	).From(htb.table+" AS c").Join(
		htb.cteName+" AS "+cteAlias,
		fmt.Sprintf("c.%s = tree.%s", q(htb.parentCol), q(htb.idCol)),
	).Where(
		fmt.Sprintf("tree.%s < %d", q(htb.depthCol), maxDepth),
		fmt.Sprintf("tree.%s = 0", q(htb.cycleCol)),
	)

	union := flavor.NewUnionBuilder().UnionAll(anchor, recursive)

	return flavor.NewCTEQueryBuilder().
		Table(htb.cteName, htb.Cols()...).
		As(union), nil
}

// CTEQuery builds the standalone recursive CTE query.
// It can be attached to any WITH RECURSIVE clause.
func (htb *HierarchyTreeBuilder) CTEQuery() (*CTEQueryBuilder, error) {
	return htb.buildCTEQuery(htb.flavor)
}

// CTEBuilder wraps the recursive query in a WITH RECURSIVE builder ready to
// be attached to a SELECT/UPDATE/DELETE statement.
func (htb *HierarchyTreeBuilder) CTEBuilder() (*CTEBuilder, error) {
	cteQuery, err := htb.buildCTEQuery(htb.flavor)
	if err != nil {
		return nil, err
	}

	return htb.flavor.NewCTEBuilder().WithRecursive(cteQuery), nil
}

// JoinFactTable builds a SELECT over the tree CTE joined to factTable.
//
// factJoinExpr is the ON expression linking a tree row to a fact row, e.g.
// "f.category_id = cat_tree.id". The tree CTE drives the FROM clause, so a
// LEFT JOIN keeps tree nodes without facts visible; pass an inner Join via
// the returned SelectBuilder if only matched facts are wanted. Because every
// tree row carries its root id, grouping by the root id column aggregates
// facts per root without any cross-root mixing.
//
// Rows flagged is_cycle = 1 repeat an already-visited node, so they are
// excluded from the fact join by default to avoid double counting a fact;
// use CTEQuery/CTEBuilder to inspect the raw marker rows for diagnostics.
// selectCols may reference tree columns and fact columns.
func (htb *HierarchyTreeBuilder) JoinFactTable(factTable, factAlias, factJoinExpr string, selectCols ...string) (*SelectBuilder, error) {
	cteBuilder, err := htb.CTEBuilder()
	if err != nil {
		return nil, err
	}

	sb := cteBuilder.Select(selectCols...).
		From(htb.cteName).
		JoinWithOption(LeftJoin, factTable+" AS "+factAlias, factJoinExpr).
		Where(htb.cteName + "." + htb.cycleCol + " = 0")

	return sb, nil
}

// String returns the compiled WITH RECURSIVE sql string.
// It panics if the builder is not configured correctly.
func (htb *HierarchyTreeBuilder) String() string {
	sql, _ := htb.Build()
	return sql
}

// Build returns the compiled WITH RECURSIVE sql string and its args.
// It panics if the builder is not configured correctly, e.g. no root id
// or a negative max depth; use BuildE to receive the error instead.
func (htb *HierarchyTreeBuilder) Build() (sql string, args []interface{}) {
	return htb.BuildWithFlavor(htb.flavor)
}

// BuildE behaves like Build but returns a configuration error instead of
// panicking.
func (htb *HierarchyTreeBuilder) BuildE() (sql string, args []interface{}, err error) {
	return htb.BuildWithFlavorE(htb.flavor)
}

// BuildWithFlavor builds the recursive CTE with the specified flavor and
// initial arguments. It panics on configuration errors; use
// BuildWithFlavorE to receive the error instead.
func (htb *HierarchyTreeBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	s, a, err := htb.BuildWithFlavorE(flavor, initialArg...)
	if err != nil {
		panic(err)
	}

	return s, a
}

// BuildWithFlavorE behaves like BuildWithFlavor but returns a configuration
// error instead of panicking.
func (htb *HierarchyTreeBuilder) BuildWithFlavorE(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}, err error) {
	cteQuery, err := htb.buildCTEQuery(flavor)
	if err != nil {
		return "", nil, err
	}

	cteBuilder := flavor.NewCTEBuilder().WithRecursive(cteQuery)
	s, a := cteBuilder.BuildWithFlavor(flavor, initialArg...)
	return s, a, nil
}
