// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

func ExampleInsertInto() {
	sql, args := InsertInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleInsertIgnoreInto() {
	sql, args := InsertIgnoreInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT IGNORE INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleReplaceInto() {
	sql, args := ReplaceInto("demo.user").
		Cols("id", "name", "status").
		Values(4, "Sample", 2).
		Build()

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// REPLACE INTO demo.user (id, name, status) VALUES (?, ?, ?)
	// [4 Sample 2]
}

func ExampleInsertBuilder() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_flavorOracle() {
	ib := Oracle.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name", "status")
	ib.Values(1, "Huan Du", 1)
	ib.Values(2, "Charmy Liu", 1)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT ALL INTO demo.user (id, name, status) VALUES (:1, :2, :3) INTO demo.user (id, name, status) VALUES (:4, :5, :6) SELECT 1 from DUAL
	// [1 Huan Du 1 2 Charmy Liu 1]
}

func ExampleInsertBuilder_insertIgnore() {
	ib := NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT IGNORE INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_postgres() {
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at) VALUES ($1, $2, $3, UNIX_TIMESTAMP(NOW())), ($4, $5, $6, $7) ON CONFLICT DO NOTHING
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_sqlite() {
	ib := SQLite.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT OR IGNORE INTO demo.user (id, name, status, created_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_insertIgnore_clickhouse() {
	ib := ClickHouse.NewInsertBuilder()
	ib.InsertIgnoreInto("demo.user")
	ib.Cols("id", "name", "status", "created_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, status, created_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_replaceInto() {
	ib := NewInsertBuilder()
	ib.ReplaceInto("demo.user")
	ib.Cols("id", "name", "status", "created_at", "updated_at")
	ib.Values(1, "Huan Du", 1, Raw("UNIX_TIMESTAMP(NOW())"))
	ib.Values(2, "Charmy Liu", 1, 1234567890)

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// REPLACE INTO demo.user (id, name, status, created_at, updated_at) VALUES (?, ?, ?, UNIX_TIMESTAMP(NOW())), (?, ?, ?, ?)
	// [1 Huan Du 1 2 Charmy Liu 1 1234567890]
}

func ExampleInsertBuilder_SQL() {
	ib := NewInsertBuilder()
	ib.SQL("/* before */")
	ib.InsertInto("demo.user")
	ib.SQL("PARTITION (p0)")
	ib.Cols("id", "name", "status", "created_at")
	ib.SQL("/* after cols */")
	ib.Values(3, "Shawn Du", 1, 1234567890)
	ib.SQL(ib.Var(Build("ON DUPLICATE KEY UPDATE status = $?", 1)))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// /* before */ INSERT INTO demo.user PARTITION (p0) (id, name, status, created_at) /* after cols */ VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE status = ?
	// [3 Shawn Du 1 1234567890 1]
}

func ExampleInsertBuilder_subSelect() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = ?
	// [1]
}

func ExampleInsertBuilder_subSelect_oracle() {
	ib := Oracle.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = :1
	// [1]
}

func ExampleInsertBuilder_subSelect_informix() {
	ib := Informix.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	sb := ib.Select("id", "name").From("demo.test")
	sb.Where(sb.EQ("id", 1))

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name) SELECT id, name FROM demo.test WHERE id = ?
	// [1]
}

func ExampleInsertBuilder_NumValue() {
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name")
	ib.Values(1, "Huan Du")
	ib.Values(2, "Charmy Liu")

	// Count the number of values.
	fmt.Println(ib.NumValue())

	// Output:
	// 2
}

func ExampleInsertBuilder_Returning() {
	sql, args := InsertInto("user").
		Cols("name").Values("Huan Du").
		Returning("id").
		BuildWithFlavor(PostgreSQL)

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO user (name) VALUES ($1) RETURNING id
	// [Huan Du]
}

func TestInsertBuilderReturning(test *testing.T) {
	a := assert.New(test)
	ib := InsertInto("user").
		Cols("name").Values("Huan Du").
		Returning("id")

	sql, _ := ib.BuildWithFlavor(MySQL)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(PostgreSQL)
	a.Equal("INSERT INTO user (name) VALUES ($1) RETURNING id", sql)

	sql, _ = ib.BuildWithFlavor(SQLite)
	a.Equal("INSERT INTO user (name) VALUES (?) RETURNING id", sql)

	sql, _ = ib.BuildWithFlavor(SQLServer)
	a.Equal("INSERT INTO user (name) OUTPUT INSERTED.id VALUES (@p1)", sql)

	sql, _ = ib.BuildWithFlavor(CQL)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(ClickHouse)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(Presto)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)

	sql, _ = ib.BuildWithFlavor(Oracle)
	a.Equal("INSERT INTO user (name) VALUES (:1)", sql)

	sql, _ = ib.BuildWithFlavor(Informix)
	a.Equal("INSERT INTO user (name) VALUES (?)", sql)
}

func TestInsertBuilderGetFlavor(t *testing.T) {
	a := assert.New(t)
	ib := newInsertBuilder()

	ib.SetFlavor(PostgreSQL)
	flavor := ib.Flavor()
	a.Equal(PostgreSQL, flavor)

	ibClick := ClickHouse.NewInsertBuilder()
	flavor = ibClick.Flavor()
	a.Equal(ClickHouse, flavor)
}

func TestIssue200(t *testing.T) {
	a := assert.New(t)
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertIgnoreInto("table")
	ib.Cols("data")
	sb := ib.Select("id", "data").From("table")
	sb.Where(sb.Equal("id", 1))

	query, _ := ib.Build()
	a.Equal(query, "INSERT INTO table (data) SELECT id, data FROM table WHERE id = $1 ON CONFLICT DO NOTHING")
}

func TestInsertBuilderClone(t *testing.T) {
	a := assert.New(t)

	ib := InsertInto("demo.user").Cols("id", "name")
	ib.Values(1, "A")

	clone := ib.Clone()

	s1, args1 := ib.Build()
	s2, args2 := clone.Build()
	a.Equal(s1, s2)
	a.Equal(args1, args2)

	// mutate clone and verify original unchanged
	clone.Values(2, "B")
	a.NotEqual(ib.String(), clone.String())
}

func ExampleInsertBuilder_onConflictVersion_postgres() {
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "shard", "name", "status", "version")
	ib.Values(1, 7, "Huan Du", 1, 100)
	ib.OnConflict("id", "shard")
	ib.VersionCol("version")

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, shard, name, status, version) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id, shard) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version
	// [1 7 Huan Du 1 100]
}

func ExampleInsertBuilder_onConflictVersion_sqlite() {
	ib := SQLite.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "shard", "name", "status", "version")
	ib.Values(1, 7, "Huan Du", 1, 100)
	ib.OnConflict("id", "shard")
	ib.VersionCol("version")

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, shard, name, status, version) VALUES (?, ?, ?, ?, ?) ON CONFLICT (id, shard) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version
	// [1 7 Huan Du 1 100]
}

func ExampleInsertBuilder_onConflictVersion_mysql() {
	ib := MySQL.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "shard", "name", "status", "version")
	ib.Values(1, 7, "Huan Du", 1, 100)
	ib.OnConflict("id", "shard")
	ib.VersionCol("version")

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, shard, name, status, version) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name = IF(VALUES(version) > demo.user.version, VALUES(name), demo.user.name), status = IF(VALUES(version) > demo.user.version, VALUES(status), demo.user.status), version = IF(VALUES(version) > demo.user.version, VALUES(version), demo.user.version)
	// [1 7 Huan Du 1 100]
}

func ExampleInsertBuilder_onConflictVersion_returning() {
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "name", "version")
	ib.Values(1, "Huan Du", 100)
	ib.OnConflict("id")
	ib.VersionCol("version")
	ib.Returning("id", "version")

	sql, args := ib.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// INSERT INTO demo.user (id, name, version) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version RETURNING id, version
	// [1 Huan Du 100]
}

func TestInsertBuilderOnConflictMultiRow(t *testing.T) {
	a := assert.New(t)

	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user")
	ib.Cols("id", "shard", "name", "version")
	ib.Values(1, 7, "A", 100)
	ib.Values(2, 7, "B", 200)
	ib.OnConflict("id", "shard")
	ib.VersionCol("version")

	sql, args := ib.Build()
	a.Equal("INSERT INTO demo.user (id, shard, name, version) VALUES ($1, $2, $3, $4), ($5, $6, $7, $8) ON CONFLICT (id, shard) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version", sql)
	a.Equal([]interface{}{1, 7, "A", 100, 2, 7, "B", 200}, args)
}

func TestInsertBuilderOnConflictClone(t *testing.T) {
	a := assert.New(t)

	ib := MySQL.NewInsertBuilder()
	ib.InsertInto("demo.user").
		Cols("id", "name", "version").
		Values(1, "A", 100).
		OnConflict("id").
		VersionCol("version")

	cloned := ib.Clone()

	s1, args1 := ib.Build()
	s2, args2 := cloned.Build()
	a.Equal(s1, s2)
	a.Equal(args1, args2)
}

func TestInsertBuilderOnConflictPanics(t *testing.T) {
	newBuilder := func(flavor Flavor) *InsertBuilder {
		return flavor.NewInsertBuilder().
			InsertInto("demo.user").
			Cols("id", "shard", "name", "version")
	}

	cases := []struct {
		name string
		run  func()
	}{
		{
			name: "version col without conflict key",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).VersionCol("version").Build()
			},
		},
		{
			name: "conflict key without version col",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).OnConflict("id").Build()
			},
		},
		{
			name: "conflict col not in cols",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).
					OnConflict("id", "missing").VersionCol("version").Build()
			},
		},
		{
			name: "version col not in cols",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).
					OnConflict("id").VersionCol("missing").Build()
			},
		},
		{
			name: "version column repeated among conflict cols",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).
					OnConflict("id", "version").VersionCol("version").Build()
			},
		},
		{
			name: "duplicated conflict col",
			run: func() {
				newBuilder(PostgreSQL).Values(1, 7, "A", 100).
					OnConflict("id", "id").VersionCol("version").Build()
			},
		},
		{
			name: "combined with insert ignore",
			run: func() {
				ib := PostgreSQL.NewInsertBuilder().
					InsertIgnoreInto("demo.user").
					Cols("id", "name", "version").
					Values(1, "A", 100).
					OnConflict("id").VersionCol("version")
				ib.Build()
			},
		},
		{
			name: "combined with replace",
			run: func() {
				newBuilder(MySQL).Values(1, 7, "A", 100).
					ReplaceInto("demo.user").
					OnConflict("id").VersionCol("version").Build()
			},
		},
		{
			name: "combined with select insert",
			run: func() {
				ib := PostgreSQL.NewInsertBuilder().
					InsertInto("demo.user").
					Cols("id", "name", "version")
				sb := ib.Select("id", "name", "version").From("demo.src")
				sb.Where(sb.EQ("id", 1))
				ib.OnConflict("id").VersionCol("version").Build()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected a panic for %s, got none", tc.name)
				}
			}()
			tc.run()
		})
	}
}

func TestInsertBuilderOnConflictUnsupportedFlavor(t *testing.T) {
	unsupported := []Flavor{SQLServer, CQL, ClickHouse, Presto, Oracle, Informix, Doris}

	for _, flavor := range unsupported {
		func(flavor Flavor) {
			t.Run(flavor.String(), func(t *testing.T) {
				defer func() {
					r := recover()
					if r == nil {
						t.Fatalf("expected a panic on flavor %s, got none", flavor)
					}

					err, ok := r.(error)
					if !ok || !errors.Is(err, ErrInsertUpsertNotImplemented) {
						t.Fatalf("expected ErrInsertUpsertNotImplemented on flavor %s, got %v", flavor, r)
					}
				}()

				flavor.NewInsertBuilder().
					InsertInto("demo.user").
					Cols("id", "name", "version").
					Values(1, "A", 100).
					OnConflict("id").
					VersionCol("version").
					Build()
			})
		}(flavor)
	}
}

func TestInsertBuilderOnConflictExplicitFlavor(t *testing.T) {
	a := assert.New(t)

	// A builder created with the default flavor can still be compiled against
	// another supported flavor through BuildWithFlavor.
	ib := NewInsertBuilder()
	ib.InsertInto("demo.user").
		Cols("id", "name", "version").
		Values(1, "A", 100).
		OnConflict("id").
		VersionCol("version")

	sql, _ := ib.BuildWithFlavor(SQLite)
	a.Equal("INSERT INTO demo.user (id, name, version) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version", sql)
}

func TestInsertBuilderOnConflictMinimalColumns(t *testing.T) {
	a := assert.New(t)

	// With only a conflict key and a version column, the version column itself
	// is the sole business column that gets updated.
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user").
		Cols("id", "version").
		Values(1, 100).
		OnConflict("id").
		VersionCol("version")

	sql, args := ib.Build()
	a.Equal("INSERT INTO demo.user (id, version) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version", sql)
	a.Equal([]interface{}{1, 100}, args)
}

func TestInsertBuilderOnConflictColumnNamesMatchCols(t *testing.T) {
	a := assert.New(t)

	// Conflict and version columns are emitted verbatim, matching the names
	// passed to Cols, without any additional escaping or quoting.
	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user").
		Cols("order_id", "region_code", "payload", "row_version").
		Values(1, "cn", "data", 100).
		OnConflict("order_id", "region_code").
		VersionCol("row_version")

	sql, _ := ib.Build()
	a.Equal("INSERT INTO demo.user (order_id, region_code, payload, row_version) VALUES ($1, $2, $3, $4) ON CONFLICT (order_id, region_code) DO UPDATE SET payload = EXCLUDED.payload, row_version = EXCLUDED.row_version WHERE EXCLUDED.row_version > demo.user.row_version", sql)
}

func TestInsertBuilderOnConflictInterpolate(t *testing.T) {
	a := assert.New(t)

	ib := PostgreSQL.NewInsertBuilder()
	ib.InsertInto("demo.user").
		Cols("id", "name", "version").
		Values(1, "Huan Du", 100).
		OnConflict("id").
		VersionCol("version")

	sql, args := ib.Build()
	interpolated, err := PostgreSQL.Interpolate(sql, args)
	a.NilError(err)
	a.Equal("INSERT INTO demo.user (id, name, version) VALUES (1, E'Huan Du', 100) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version WHERE EXCLUDED.version > demo.user.version", interpolated)
}
