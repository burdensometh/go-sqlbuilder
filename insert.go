// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
	"strings"

	"github.com/huandu/go-clone"
)

const (
	insertMarkerInit injectionMarker = iota
	insertMarkerAfterInsertInto
	insertMarkerAfterCols
	insertMarkerAfterValues
	insertMarkerAfterSelect
	insertMarkerAfterReturning
)

// NewInsertBuilder creates a new INSERT builder.
func NewInsertBuilder() *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder()
}

func newInsertBuilder() *InsertBuilder {
	args := &Args{}
	return &InsertBuilder{
		verb:      "INSERT",
		args:      args,
		injection: newInjection(),
	}
}

// Clone returns a deep copy of InsertBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (ib *InsertBuilder) Clone() *InsertBuilder {
	return clone.Clone(ib).(*InsertBuilder)
}

// InsertBuilder is a builder to build INSERT.
type InsertBuilder struct {
	verb      string
	table     string
	cols      []string
	values    [][]string
	returning []string

	// insertIgnore is true when the statement is built by InsertIgnoreInto.
	// It is mutually exclusive with the OnConflict/VersionCol upsert feature.
	insertIgnore bool

	// conflictCols are the columns of a composite conflict (business) key.
	// When it is not empty, the statement is compiled as an upsert statement
	// together with versionCol.
	conflictCols []string

	// versionCol is the column used to compare the incoming version with the
	// existing one. Business columns are overwritten only when the incoming
	// version is strictly greater than the existing version.
	versionCol string

	args *Args

	injection *injection
	marker    injectionMarker

	sbHolder string
}

// ErrInsertUpsertNotImplemented means the upsert feature driven by
// OnConflict and VersionCol is not supported by the current flavor.
var ErrInsertUpsertNotImplemented = errors.New("go-sqlbuilder: insert upsert with conflict key and version is not implemented for this flavor")

var _ Builder = new(InsertBuilder)

// InsertInto sets table name in INSERT.
func InsertInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertInto(table)
}

// InsertInto sets table name in INSERT.
func (ib *InsertBuilder) InsertInto(table string) *InsertBuilder {
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

// InsertIgnoreInto sets table name in INSERT IGNORE.
func InsertIgnoreInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertIgnoreInto(table)
}

// InsertIgnoreInto sets table name in INSERT IGNORE.
func (ib *InsertBuilder) InsertIgnoreInto(table string) *InsertBuilder {
	ib.args.Flavor.PrepareInsertIgnore(table, ib)
	return ib
}

// ReplaceInto sets table name and changes the verb of ib to REPLACE.
// REPLACE INTO is a MySQL extension to the SQL standard.
func ReplaceInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().ReplaceInto(table)
}

// ReplaceInto sets table name and changes the verb of ib to REPLACE.
// REPLACE INTO is a MySQL extension to the SQL standard.
func (ib *InsertBuilder) ReplaceInto(table string) *InsertBuilder {
	ib.verb = "REPLACE"
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

// Cols sets columns in INSERT.
func (ib *InsertBuilder) Cols(col ...string) *InsertBuilder {
	ib.cols = EscapeAll(col...)
	ib.marker = insertMarkerAfterCols
	return ib
}

// Select returns a new SelectBuilder to build a SELECT statement inside the INSERT INTO.
func (isb *InsertBuilder) Select(col ...string) *SelectBuilder {
	sb := Select(col...)
	isb.sbHolder = isb.args.Add(sb)
	return sb
}

// Values adds a list of values for a row in INSERT.
func (ib *InsertBuilder) Values(value ...interface{}) *InsertBuilder {
	placeholders := make([]string, 0, len(value))

	for _, v := range value {
		placeholders = append(placeholders, ib.args.Add(v))
	}

	ib.values = append(ib.values, placeholders)
	ib.marker = insertMarkerAfterValues
	return ib
}

// Returning sets returning columns.
// For DBMS that doesn't support RETURNING, e.g. MySQL, it will be ignored.
func (ib *InsertBuilder) Returning(col ...string) *InsertBuilder {
	ib.returning = col
	ib.marker = insertMarkerAfterReturning
	return ib
}

// NumValue returns the number of values to insert.
func (ib *InsertBuilder) NumValue() int {
	return len(ib.values)
}

// OnConflict declares the columns of a composite conflict (business) key.
// Column names follow the same escaping rules as Cols.
// Together with VersionCol, it turns the statement into an upsert statement:
//
//   - A row whose conflict key does not exist is inserted as usual, so a new
//     key is always written.
//   - On a conflicting key, business columns are overwritten only when the
//     incoming version (the value of versionCol) is strictly greater than the
//     version stored in the table. Re-running with the same version changes
//     nothing and never inserts another row.
//   - Conflict key columns themselves are never part of the UPDATE assignment,
//     so an existing business key is never modified.
//
// The table must have a unique constraint or a unique index covering all the
// conflict columns. The conflict columns must also be listed in Cols.
//
// This upsert feature is only supported on MySQL, PostgreSQL and SQLite.
// Calling Build on any other flavor panics with ErrInsertUpsertNotImplemented.
// It cannot be combined with InsertIgnoreInto, ReplaceInto or a SELECT-based
// insert.
func (ib *InsertBuilder) OnConflict(col ...string) *InsertBuilder {
	ib.conflictCols = append([]string(nil), col...)
	return ib
}

// VersionCol sets the version column used by the upsert statement declared by
// OnConflict. The column name follows the same escaping rules as Cols. The
// column must be listed in Cols and must not be one of the conflict columns.
// Business columns are overwritten only when the incoming value of this column
// is strictly greater than the existing value.
func (ib *InsertBuilder) VersionCol(col string) *InsertBuilder {
	ib.versionCol = col
	return ib
}

// String returns the compiled INSERT string.
func (ib *InsertBuilder) String() string {
	s, _ := ib.Build()
	return s
}

// Build returns compiled INSERT string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ib *InsertBuilder) Build() (sql string, args []interface{}) {
	return ib.BuildWithFlavor(ib.args.Flavor)
}

// BuildWithFlavor returns compiled INSERT string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ib *InsertBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	ib.validateUpsert(flavor)

	buf := newStringBuilder()
	ib.injection.WriteTo(buf, insertMarkerInit)

	if len(ib.values) > 1 && ib.args.Flavor == Oracle {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" ALL")

		for _, v := range ib.values {
			if len(ib.table) > 0 {
				buf.WriteString(" INTO ")
				buf.WriteString(ib.table)
			}
			ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)
			if len(ib.cols) > 0 {
				buf.WriteLeadingString("(")
				buf.WriteStrings(ib.cols, ", ")
				buf.WriteString(")")

				ib.injection.WriteTo(buf, insertMarkerAfterCols)
			}

			buf.WriteLeadingString("VALUES ")
			values := make([]string, 0, len(ib.values))
			values = append(values, fmt.Sprintf("(%v)", strings.Join(v, ", ")))
			buf.WriteStrings(values, ", ")
		}

		buf.WriteString(" SELECT 1 from DUAL")

		ib.injection.WriteTo(buf, insertMarkerAfterValues)

		return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
	}

	if len(ib.table) > 0 {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" INTO ")
		buf.WriteString(ib.table)
	}

	ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)

	if len(ib.cols) > 0 {
		buf.WriteLeadingString("(")
		buf.WriteStrings(ib.cols, ", ")
		buf.WriteString(")")

		ib.injection.WriteTo(buf, insertMarkerAfterCols)
	}

	if flavor == SQLServer {
		if len(ib.returning) > 0 {
			buf.WriteLeadingString("OUTPUT ")
			buf.WriteStringsPrefixed("INSERTED.", ib.returning, ", ")
		}

		ib.injection.WriteTo(buf, insertMarkerAfterReturning)
	}

	if ib.sbHolder != "" {
		buf.WriteString(" ")
		buf.WriteString(ib.sbHolder)

		ib.injection.WriteTo(buf, insertMarkerAfterSelect)
	} else if len(ib.values) > 0 {
		buf.WriteLeadingString("VALUES ")
		values := make([]string, 0, len(ib.values))

		for _, v := range ib.values {
			values = append(values, fmt.Sprintf("(%v)", strings.Join(v, ", ")))
		}

		buf.WriteStrings(values, ", ")
	}

	ib.injection.WriteTo(buf, insertMarkerAfterValues)

	if len(ib.conflictCols) > 0 {
		buf.WriteLeadingString(ib.buildOnConflict(flavor))
	}

	if flavor == PostgreSQL || flavor == SQLite {
		if len(ib.returning) > 0 {
			buf.WriteLeadingString("RETURNING ")
			buf.WriteStrings(ib.returning, ", ")
		}

		ib.injection.WriteTo(buf, insertMarkerAfterReturning)
	}

	return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// hasUpsert reports whether an upsert statement is declared by OnConflict.
func (ib *InsertBuilder) hasUpsert() bool {
	return len(ib.conflictCols) > 0 || len(ib.versionCol) > 0
}

// validateUpsert validates the upsert configuration declared by OnConflict and
// VersionCol. It panics on an invalid usage or an unsupported flavor, following
// the same convention as PrepareInsertIgnore.
func (ib *InsertBuilder) validateUpsert(flavor Flavor) {
	if !ib.hasUpsert() {
		return
	}

	if len(ib.conflictCols) == 0 {
		panic(fmt.Errorf("go-sqlbuilder: VersionCol requires OnConflict to declare the conflict key"))
	}

	if len(ib.versionCol) == 0 {
		panic(fmt.Errorf("go-sqlbuilder: OnConflict requires VersionCol to declare the version column"))
	}

	if ib.verb != "INSERT" {
		panic(fmt.Errorf("go-sqlbuilder: OnConflict cannot be combined with %s", ib.verb))
	}

	if ib.insertIgnore {
		panic(fmt.Errorf("go-sqlbuilder: OnConflict cannot be combined with InsertIgnoreInto"))
	}

	if ib.sbHolder != "" {
		panic(fmt.Errorf("go-sqlbuilder: OnConflict does not support a SELECT-based insert"))
	}

	colSet := make(map[string]struct{}, len(ib.cols))
	for _, col := range ib.cols {
		colSet[col] = struct{}{}
	}

	if len(colSet) == 0 {
		panic(fmt.Errorf("go-sqlbuilder: OnConflict requires columns declared by Cols"))
	}

	seen := make(map[string]struct{}, len(ib.conflictCols))
	for _, key := range ib.conflictCols {
		if _, ok := colSet[key]; !ok {
			panic(fmt.Errorf("go-sqlbuilder: conflict column %q must be listed in Cols", key))
		}

		if _, dup := seen[key]; dup {
			panic(fmt.Errorf("go-sqlbuilder: conflict column %q is duplicated", key))
		}

		seen[key] = struct{}{}
	}

	if _, ok := colSet[ib.versionCol]; !ok {
		panic(fmt.Errorf("go-sqlbuilder: version column %q must be listed in Cols", ib.versionCol))
	}

	if _, ok := seen[ib.versionCol]; ok {
		panic(fmt.Errorf("go-sqlbuilder: version column %q cannot be a conflict column", ib.versionCol))
	}

	switch flavor {
	case MySQL, PostgreSQL, SQLite:
		// Supported flavors.
	default:
		panic(fmt.Errorf("%w: %s", ErrInsertUpsertNotImplemented, flavor.String()))
	}
}

// buildOnConflict compiles the dialect-specific upsert clause. The caller must
// have validated the configuration with validateUpsert.
func (ib *InsertBuilder) buildOnConflict(flavor Flavor) string {
	// Business columns are every column except the conflict key columns.
	// They include the version column and are overwritten only when the
	// incoming version is strictly greater than the existing version.
	conflictSet := make(map[string]struct{}, len(ib.conflictCols))
	for _, key := range ib.conflictCols {
		conflictSet[key] = struct{}{}
	}

	updateCols := make([]string, 0, len(ib.cols)-len(ib.conflictCols))
	for _, col := range ib.cols {
		if _, isKey := conflictSet[col]; !isKey {
			updateCols = append(updateCols, col)
		}
	}

	switch flavor {
	case PostgreSQL, SQLite:
		buf := newStringBuilder()
		buf.WriteString("ON CONFLICT (")
		buf.WriteStrings(ib.conflictCols, ", ")
		buf.WriteString(") DO UPDATE SET ")

		assignments := make([]string, 0, len(updateCols))
		for _, col := range updateCols {
			assignments = append(assignments, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}

		buf.WriteStrings(assignments, ", ")
		buf.WriteString(fmt.Sprintf(" WHERE EXCLUDED.%s > %s.%s", ib.versionCol, ib.table, ib.versionCol))
		return buf.String()

	case MySQL:
		buf := newStringBuilder()
		buf.WriteString("ON DUPLICATE KEY UPDATE ")

		assignments := make([]string, 0, len(updateCols))
		for _, col := range updateCols {
			// MySQL has no WHERE clause in ON DUPLICATE KEY UPDATE, so guard
			// every assignment with the version comparison. On a stale or equal
			// version each column keeps its existing value.
			assignments = append(assignments, fmt.Sprintf(
				"%s = IF(VALUES(%s) > %s.%s, VALUES(%s), %s.%s)",
				col, ib.versionCol, ib.table, ib.versionCol,
				col, ib.table, col,
			))
		}

		buf.WriteStrings(assignments, ", ")
		return buf.String()

	default:
		// Unreachable: validateUpsert rejects unsupported flavors beforehand.
		panic(fmt.Errorf("%w: %s", ErrInsertUpsertNotImplemented, flavor.String()))
	}
}

// SetFlavor sets the flavor of compiled sql.
func (ib *InsertBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ib.args.Flavor
	ib.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (ib *InsertBuilder) Flavor() Flavor {
	return ib.args.Flavor
}

// Var returns a placeholder for value.
func (ib *InsertBuilder) Var(arg interface{}) string {
	return ib.args.Add(arg)
}

// SQL adds an arbitrary sql to current position.
func (ib *InsertBuilder) SQL(sql string) *InsertBuilder {
	ib.injection.SQL(ib.marker, sql)
	return ib
}
