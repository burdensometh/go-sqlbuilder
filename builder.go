// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"

	"github.com/huandu/go-clone"
)

// Builder is a general SQL builder.
// It's used by Args to create nested SQL like the `IN` expression in
// `SELECT * FROM t1 WHERE id IN (SELECT id FROM t2)`.
type Builder interface {
	Build() (sql string, args []interface{})
	BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{})
	Flavor() Flavor
}

type compiledBuilder struct {
	args   *Args
	format string
}

var _ Builder = new(compiledBuilder)

func (cb *compiledBuilder) Build() (sql string, args []interface{}) {
	return cb.args.Compile(cb.format)
}

func (cb *compiledBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	return cb.args.CompileWithFlavor(cb.format, flavor, initialArg...)
}

// Flavor returns flavor of builder
// Always returns DefaultFlavor
func (cb *compiledBuilder) Flavor() Flavor {
	return cb.args.Flavor
}

type flavoredBuilder struct {
	builder Builder
	flavor  Flavor
}

func (fb *flavoredBuilder) Build() (sql string, args []interface{}) {
	return fb.builder.BuildWithFlavor(fb.flavor)
}

func (fb *flavoredBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	return fb.builder.BuildWithFlavor(flavor, initialArg...)
}

// Flavor returns flavor of builder
func (fb *flavoredBuilder) Flavor() Flavor {
	return fb.flavor
}

// WithFlavor creates a new Builder based on builder with a default flavor.
func WithFlavor(builder Builder, flavor Flavor) Builder {
	return &flavoredBuilder{
		builder: builder,
		flavor:  flavor,
	}
}

// Fork creates n independent deep copies of builder. All CTEs, WHERE clauses
// and the args scope are cloned together with builder, so that every copy can
// be customized independently, e.g. adding partition-specific filters, without
// affecting the original builder or any other copy.
//
// Fork is the entry point to generate a set of partition queries from the
// same assembled base query:
//
//	base := sqlbuilder.Select("id", "status").From("orders").Where(...)
//	queries := sqlbuilder.Fork(base, len(reports))
//
//	for i, report := range reports {
//		q := queries[i]
//		q.Where(q.Equal("status", report.Status)) // report-specific filters
//	}
//
// Each forked query is a Builder and can be consumed individually, e.g. by
// an outer INSERT ... SELECT built with InsertBuilder.SelectBuilder.
//
// If n is less than or equal to 0, Fork returns nil.
func Fork[B Builder](builder B, n int) []B {
	if n <= 0 {
		return nil
	}

	builders := make([]B, n)

	for i := range builders {
		builders[i] = clone.Clone(builder).(B)
	}

	return builders
}

// Buildf creates a Builder from a format string using `fmt.Sprintf`-like syntax.
// As all arguments will be converted to a string internally, e.g. "$0",
// only `%v` and `%s` are valid.
func Buildf(format string, arg ...interface{}) Builder {
	args := &Args{
		Flavor: DefaultFlavor,
	}
	vars := make([]interface{}, 0, len(arg))

	for _, a := range arg {
		vars = append(vars, args.Add(a))
	}

	return &compiledBuilder{
		args:   args,
		format: fmt.Sprintf(Escape(format), vars...),
	}
}

// Build creates a Builder from a format string.
// The format string uses special syntax to represent arguments.
// See doc in `Args#Compile` for syntax details.
func Build(format string, arg ...interface{}) Builder {
	args := &Args{
		Flavor: DefaultFlavor,
	}

	for _, a := range arg {
		args.Add(a)
	}

	return &compiledBuilder{
		args:   args,
		format: format,
	}
}

// BuildNamed creates a Builder from a format string.
// The format string uses `${key}` to refer the value of named by key.
func BuildNamed(format string, named map[string]interface{}) Builder {
	args := &Args{
		Flavor:    DefaultFlavor,
		onlyNamed: true,
	}

	for n, v := range named {
		args.Add(Named(n, v))
	}

	return &compiledBuilder{
		args:   args,
		format: format,
	}
}
