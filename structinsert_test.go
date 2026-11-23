// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"testing"

	"github.com/huandu/go-assert"
)

type structBatchUser struct {
	ID       int    `db:"id" fieldtag:"pk"`
	Name     string `db:"name"`
	Nickname string `db:"nickname" fieldopt:"omitempty"`
}

func TestStructInsertIntoBatchesSplitsByPlaceholderLimit(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	users := []interface{}{
		&structBatchUser{ID: 1, Name: "a"},
		&structBatchUser{ID: 2, Name: "b"},
		&structBatchUser{ID: 3, Name: "c"},
		&structBatchUser{ID: 4, Name: "d"},
		&structBatchUser{ID: 5, Name: "e"},
	}

	// Each row binds 2 placeholders (nickname is empty and omitted).
	batches, err := st.InsertIntoBatches("t", 4, users...)
	a.NilError(err)
	a.Equal(len(batches), 3)

	a.Equal(batches[0].String(), "INSERT INTO t (id, name) VALUES (?, ?), (?, ?)")
	a.Equal(batches[0].Indices, []int{0, 1})

	a.Equal(batches[1].String(), "INSERT INTO t (id, name) VALUES (?, ?), (?, ?)")
	a.Equal(batches[1].Indices, []int{2, 3})

	a.Equal(batches[2].String(), "INSERT INTO t (id, name) VALUES (?, ?)")
	a.Equal(batches[2].Indices, []int{4})

	_, args0 := batches[0].Build()
	a.Equal(args0, []interface{}{1, "a", 2, "b"})
	_, args1 := batches[1].Build()
	a.Equal(args1, []interface{}{3, "c", 4, "d"})
	_, args2 := batches[2].Build()
	a.Equal(args2, []interface{}{5, "e"})
}

func TestStructInsertIntoBatchesUsesColumnsActuallyWritten(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	users := []interface{}{
		&structBatchUser{ID: 1, Name: "a", Nickname: "aa"},
		&structBatchUser{ID: 2, Name: "b", Nickname: "bb"},
		&structBatchUser{ID: 3, Name: "c"},
	}

	// First two rows bind 3 columns each, the last binds 2. The limit of 6
	// fits the two full rows but not the third, whose 2 extra columns would
	// push the statement to 8 placeholders.
	batches, err := st.InsertIntoBatches("t", 6, users...)
	a.NilError(err)
	a.Equal(len(batches), 2)

	a.Equal(batches[0].String(), "INSERT INTO t (id, name, nickname) VALUES (?, ?, ?), (?, ?, ?)")
	a.Equal(batches[0].Indices, []int{0, 1})

	a.Equal(batches[1].String(), "INSERT INTO t (id, name) VALUES (?, ?)")
	a.Equal(batches[1].Indices, []int{2})
}

func TestStructInsertIntoBatchesKeepsSourceIndices(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	other := struct{ ID int }{ID: 99}

	values := []interface{}{
		&structBatchUser{ID: 1, Name: "a"},
		&other,
		&structBatchUser{ID: 2, Name: "b"},
		&structBatchUser{ID: 3, Name: "c"},
	}

	batches, err := st.InsertIntoBatches("t", 2, values...)
	a.NilError(err)
	a.Equal(len(batches), 3)

	// Non-struct items are skipped, but the surviving records keep the index
	// they had in the original value list.
	a.Equal(batches[0].Indices, []int{0})
	a.Equal(batches[1].Indices, []int{2})
	a.Equal(batches[2].Indices, []int{3})
}

func TestStructInsertIntoBatchesSingleRecordOverLimit(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	other := struct{ ID int }{ID: 99}

	values := []interface{}{
		&structBatchUser{ID: 1, Name: "a"},
		&other,
		&structBatchUser{ID: 2, Name: "b", Nickname: "bb"},
	}

	batches, err := st.InsertIntoBatches("t", 2, values...)
	a.Assert(batches == nil)
	a.Assert(err != nil)
	a.Assert(errors.Is(err, ErrInsertRowExceedsPlaceholderLimit))

	var limitErr *InsertRowExceedsPlaceholderLimitError
	a.Assert(errors.As(err, &limitErr))
	a.Equal(limitErr.Index, 2)
	a.Equal(limitErr.Placeholders, 3)
	a.Equal(limitErr.Limit, 2)
}

func TestStructInsertIntoBatchesEmptyInput(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	batches, err := st.InsertIntoBatches("t", 100)
	a.NilError(err)
	a.Equal(len(batches), 0)

	// Items whose type does not match are ignored; no statement is produced.
	batches, err = st.InsertIntoBatches("t", 100, struct{ ID int }{ID: 1})
	a.NilError(err)
	a.Equal(len(batches), 0)
}

func TestStructInsertIntoBatchesNoLimit(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(PostgreSQL)

	users := make([]interface{}, 0, 1000)

	for i := 0; i < 1000; i++ {
		users = append(users, &structBatchUser{ID: i, Name: "user"})
	}

	batches, err := st.InsertIntoBatches("t", 0, users...)
	a.NilError(err)
	a.Equal(len(batches), 1)
	a.Equal(len(batches[0].Indices), 1000)

	sql, args := batches[0].Build()
	a.Equal(sql[:len("INSERT INTO t (id, name) VALUES ($1, $2)")], "INSERT INTO t (id, name) VALUES ($1, $2)")
	a.Equal(len(args), 2000)
}

func TestStructInsertIntoBatchesVariants(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	user := &structBatchUser{ID: 1, Name: "a"}

	ignore, err := st.InsertIgnoreIntoBatches("t", 10, user)
	a.NilError(err)
	a.Equal(ignore[0].String(), "INSERT IGNORE INTO t (id, name) VALUES (?, ?)")

	replace, err := st.ReplaceIntoBatches("t", 10, user)
	a.NilError(err)
	a.Equal(replace[0].String(), "REPLACE INTO t (id, name) VALUES (?, ?)")
}

func TestStructInsertIntoBatchesForTag(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL).WithTag("pk")

	user := &structBatchUser{ID: 1, Name: "a"}

	batches, err := st.InsertIntoBatches("t", 10, user)
	a.NilError(err)
	a.Equal(batches[0].String(), "INSERT INTO t (id) VALUES (?)")
}

func TestStructInsertIntoBatchesProperty(t *testing.T) {
	a := assert.New(t)
	st := NewStruct(new(structBatchUser)).For(MySQL)

	// Deterministic pseudo-random data: rows vary which optional fields they set.
	values := make([]interface{}, 0, 200)
	want := make([]int, 0, 200)

	for i := 0; i < 200; i++ {
		u := &structBatchUser{ID: i, Name: "n"}

		if i%3 != 0 {
			u.Nickname = "nick"
		}

		values = append(values, u)
		want = append(want, i)
	}

	// Every record binds at least id and name, and some bind nickname too, so
	// limits below 3 cannot fit every record and are checked separately.
	for _, limit := range []int{3, 4, 5, 7, 11, 100, 1000} {
		batches, err := st.InsertIntoBatches("t", limit, values...)
		a.NilError(err)

		got := make([]int, 0, len(values))

		for _, b := range batches {
			a.Assert(len(b.Indices) > 0)

			// The statement must not exceed the limit.
			cols := len(b.values[0])
			a.Assert(cols*len(b.Indices) <= limit)

			// Every row in the batch must bind every column of the batch.
			for _, row := range b.values {
				a.Equal(len(row), cols)
			}

			got = append(got, b.Indices...)
		}

		// All records appear exactly once, in source order.
		a.Equal(got, want)
	}
}
