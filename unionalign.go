// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
)

// Sentinel errors reported by UnionAllWithSchema.
var (
	// ErrUnionNoTargetColumn means the target column definition is empty.
	ErrUnionNoTargetColumn = errors.New("go-sqlbuilder: union all with schema requires at least one target column")

	// ErrUnionNoSource means no source builder is provided.
	ErrUnionNoSource = errors.New("go-sqlbuilder: union all with schema requires at least one source")

	// ErrUnionMissingRequiredColumn means a required (non-optional) target column is not provided by a source.
	ErrUnionMissingRequiredColumn = errors.New("go-sqlbuilder: source is missing a required target column")

	// ErrUnionNilBuilder means a source is created without a Builder.
	ErrUnionNilBuilder = errors.New("go-sqlbuilder: union source builder must not be nil")

	// ErrUnionEmptyName means a target column or a source has an empty name.
	ErrUnionEmptyName = errors.New("go-sqlbuilder: target column name and source name must not be empty")

	// ErrUnionOptionalColumnNoType means an optional target column has no type, so a typed NULL cannot be generated.
	ErrUnionOptionalColumnNoType = errors.New("go-sqlbuilder: optional target column must declare a type for the typed NULL")

	// ErrUnionDuplicateColumn means the target column definition contains duplicate column names.
	ErrUnionDuplicateColumn = errors.New("go-sqlbuilder: duplicate target column in union all with schema")

	// ErrUnionUnknownColumn means a source maps a column that is not declared in the target column definition.
	ErrUnionUnknownColumn = errors.New("go-sqlbuilder: source maps an unknown target column")
)

// UnionColumn describes one column of the target schema used to align UNION ALL sources.
//
// Name is the target column name. Every source projection is aligned to this name and order.
// Type is only mandatory for optional columns: when a source does not provide an optional
// column, the builder fills the gap with `CAST(NULL AS <Type>) AS <Name>`.
// Set Optional to true for nullable columns that may be absent from some sources.
type UnionColumn struct {
	Name     string
	Type     string
	Optional bool
}

// UnionSource describes one source query and how its projection maps to the target columns.
//
// Name identifies the source in error messages, e.g. the table name or a business source label.
// Builder is the source query. It is wrapped in a derived table, so it can be any Builder.
// Columns maps target column names to the expression to read from the wrapped source query,
// e.g. "old_col" or "src.amount / 100". Sources may list columns in any order and only need
// to provide the required target columns.
type UnionSource struct {
	Name    string
	Builder Builder
	Columns map[string]string
}

// unionSourceAlias is the fixed alias of the derived table wrapping every source.
// All references in UnionSource.Columns are resolved against this alias.
const unionSourceAlias = "src"

// UnionAllWithSchema creates a UnionBuilder which aligns every source projection to the given
// target columns before joining them with UNION ALL.
//
// Field order differences between sources are handled by reordering each projection to match
// cols. An optional column missing from a source is filled with a typed NULL, i.e.
// `CAST(NULL AS <Type>) AS <Name>`, so the column type stays explicit. If a required column is
// missing, an error wrapping ErrUnionMissingRequiredColumn is returned and the message names
// both the source and the column.
//
// The returned UnionBuilder can be embedded into another query like any Builder, e.g. as a
// derived table in a SELECT that continues to aggregate by a business region.
func UnionAllWithSchema(cols []UnionColumn, sources []UnionSource) (*UnionBuilder, error) {
	ub := NewUnionBuilder()

	if err := ub.UnionAllWithSchema(cols, sources); err != nil {
		return nil, err
	}

	return ub, nil
}

// UnionAllWithSchema aligns every source projection to the given target columns and joins them
// with UNION ALL. See the package-level UnionAllWithSchema function for the full behavior.
func (ub *UnionBuilder) UnionAllWithSchema(cols []UnionColumn, sources []UnionSource) error {
	wrappers, err := buildUnionSchemaWrappers(ub, cols, sources)
	if err != nil {
		return err
	}

	ub.UnionAll(wrappers...)
	return nil
}

// buildUnionSchemaWrappers validates the target schema and builds one aligned projection
// SelectBuilder per source.
func buildUnionSchemaWrappers(ub *UnionBuilder, cols []UnionColumn, sources []UnionSource) ([]Builder, error) {
	if len(cols) == 0 {
		return nil, ErrUnionNoTargetColumn
	}

	indexByName := make(map[string]int, len(cols))

	for i, col := range cols {
		if col.Name == "" {
			return nil, fmt.Errorf("%w: target column at index %d", ErrUnionEmptyName, i)
		}

		if _, ok := indexByName[col.Name]; ok {
			return nil, fmt.Errorf("%w: %s", ErrUnionDuplicateColumn, col.Name)
		}

		indexByName[col.Name] = i

		if col.Optional && col.Type == "" {
			return nil, fmt.Errorf("%w: %s", ErrUnionOptionalColumnNoType, col.Name)
		}
	}

	if len(sources) == 0 {
		return nil, ErrUnionNoSource
	}

	wrappers := make([]Builder, 0, len(sources))

	for _, source := range sources {
		if source.Name == "" {
			return nil, fmt.Errorf("%w: source at index %d", ErrUnionEmptyName, len(wrappers))
		}

		if source.Builder == nil {
			return nil, fmt.Errorf("%w: source %s", ErrUnionNilBuilder, source.Name)
		}

		for colName := range source.Columns {
			if _, ok := indexByName[colName]; !ok {
				return nil, fmt.Errorf("%w: source %s, column %s", ErrUnionUnknownColumn, source.Name, colName)
			}
		}

		wrapper := ub.Flavor().NewSelectBuilder()
		projections := make([]string, len(cols))

		for i, col := range cols {
			expr, ok := source.Columns[col.Name]

			if !ok {
				if !col.Optional {
					return nil, fmt.Errorf("%w: source %s, column %s", ErrUnionMissingRequiredColumn, source.Name, col.Name)
				}

				// Fill the missing nullable column with an explicitly typed NULL.
				expr = fmt.Sprintf("CAST(NULL AS %s)", col.Type)
			}

			projections[i] = wrapper.As(expr, col.Name)
		}

		wrapper.Select(projections...)
		wrapper.From(wrapper.BuilderAs(source.Builder, unionSourceAlias))
		wrappers = append(wrappers, wrapper)
	}

	return wrappers, nil
}
