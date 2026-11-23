// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/huandu/go-assert"
)

// inventoryPatch mirrors a partial update from an inventory source.
//
// Pointer fields represent fields the source may or may not carry:
//   - nil pointer: the source didn't collect the field, so it must not be written.
//   - non-nil pointer: the source provided the field, even if it points to zero.
//   - **T with non-nil outer and nil inner: the source explicitly cleared the field,
//     which must be written as NULL.
type inventoryPatch struct {
	SKU       int64    `db:"sku_id"`
	Warehouse int64    `db:"warehouse_id"`
	Quantity  *int64   `db:"quantity" fieldtag:"stock"`
	Reason    **string `db:"reason" fieldtag:"stock"`
	Note      *string  `db:"note" fieldtag:"stock"`
	UpdatedAt int64    `db:"updated_at" fieldopt:"omitempty" fieldtag:"stock"`
}

var inventoryPatchStruct = NewStruct(new(inventoryPatch))

func TestStructUpdatePartialWritesReachedFieldsOnly(t *testing.T) {
	a := assert.New(t)

	zero := int64(0)
	cleared := (*string)(nil) // explicit clear
	updatedAt := int64(1700000000)

	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
		Quantity:  &zero,    // reached, real zero value
		Reason:    &cleared, // reached, explicit NULL
		Note:      nil,      // not reached, must be omitted
		UpdatedAt: updatedAt,
	}

	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id", "warehouse_id")
	a.NilError(err)
	a.Assert(ub != nil)

	sql, args := ub.Build()
	a.Equal(sql, "UPDATE inventory SET quantity = ?, reason = ?, updated_at = ? WHERE sku_id = ? AND warehouse_id = ?")
	a.Equal(args, []interface{}{zero, nil, updatedAt, int64(42), int64(7)})
	a.Equal(ub.NumAssignment(), 3)
}

func TestStructUpdatePartialSkipsUnreachedFields(t *testing.T) {
	a := assert.New(t)

	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
	}

	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id", "warehouse_id")

	// No valid change: a nil builder tells the caller not to touch any row.
	a.Assert(errors.Is(err, ErrUpdatePartialNoChanges))
	a.Assert(ub == nil)
}

func TestStructUpdatePartialExplicitNULLOnly(t *testing.T) {
	a := assert.New(t)

	cleared := (*string)(nil)
	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
		Reason:    &cleared,
	}

	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id", "warehouse_id")
	a.NilError(err)

	sql, args := ub.Build()
	a.Equal(sql, "UPDATE inventory SET reason = ? WHERE sku_id = ? AND warehouse_id = ?")
	a.Equal(args, []interface{}{nil, int64(42), int64(7)})
}

func TestStructUpdatePartialKeyIsNeverAssigned(t *testing.T) {
	a := assert.New(t)

	qty := int64(3)
	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
		Quantity:  &qty,
	}

	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id")
	a.NilError(err)

	sql, _ := ub.Build()
	a.Equal(sql, "UPDATE inventory SET warehouse_id = ?, quantity = ? WHERE sku_id = ?")

	// warehouse_id is not a key here, so it is written as a regular value.
	a.Equal(ub.NumAssignment(), 2)
}

func TestStructUpdatePartialRequiresUsableKey(t *testing.T) {
	a := assert.New(t)

	qty := int64(3)

	// A zero business key must not silently fall back to an unrestricted UPDATE.
	patch := &inventoryPatch{Quantity: &qty}
	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id")
	a.Assert(errors.Is(err, ErrUpdatePartialMissingKey))
	a.Assert(ub == nil)

	// An unknown key column is rejected too.
	patch = &inventoryPatch{SKU: 42, Quantity: &qty}
	ub, err = inventoryPatchStruct.UpdatePartial("inventory", patch, "not_a_column")
	a.Assert(errors.Is(err, ErrUpdatePartialMissingKey))
	a.Assert(ub == nil)

	// A missing key list is never allowed.
	ub, err = inventoryPatchStruct.UpdatePartial("inventory", patch)
	a.Assert(errors.Is(err, ErrUpdatePartialMissingKey))
	a.Assert(ub == nil)
}

func TestStructUpdatePartialTypeMismatch(t *testing.T) {
	a := assert.New(t)

	ub, err := inventoryPatchStruct.UpdatePartial("inventory", struct{ SKU int64 }{1}, "sku_id")
	a.Assert(errors.Is(err, ErrUpdatePartialTypeMismatch))
	a.Assert(ub == nil)

	ub, err = inventoryPatchStruct.UpdatePartial("inventory", nil, "sku_id")
	a.Assert(errors.Is(err, ErrUpdatePartialTypeMismatch))
	a.Assert(ub == nil)
}

func TestStructUpdatePartialRespectsTags(t *testing.T) {
	a := assert.New(t)

	qty := int64(5)
	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
		Quantity:  &qty,
		Note:      new(string), // reached, empty string must still be written
	}

	// The key columns are resolved even though they aren't tagged "stock",
	// so seller records are still scoped to the right row.
	ub, err := inventoryPatchStruct.WithTag("stock").UpdatePartial("inventory", patch, "sku_id", "warehouse_id")
	a.NilError(err)

	sql, args := ub.Build()
	a.Equal(sql, "UPDATE inventory SET quantity = ?, note = ? WHERE sku_id = ? AND warehouse_id = ?")
	a.Equal(args, []interface{}{qty, "", int64(42), int64(7)})
}

func TestStructUpdatePartialPostgreSQLPlaceholders(t *testing.T) {
	a := assert.New(t)

	qty := int64(0)
	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
		Quantity:  &qty,
	}

	ub, err := inventoryPatchStruct.For(PostgreSQL).UpdatePartial("inventory", patch, "sku_id", "warehouse_id")
	a.NilError(err)

	sql, args := ub.Build()
	a.Equal(sql, "UPDATE inventory SET quantity = $1 WHERE sku_id = $2 AND warehouse_id = $3")
	a.Equal(args, []interface{}{qty, int64(42), int64(7)})
}

func TestStructUpdatePartialDoesNotChangeUpdate(t *testing.T) {
	a := assert.New(t)

	// Existing Update semantics stay intact: a nil pointer without omitempty is
	// still written as NULL, so callers can rely on the old behavior.
	patch := &inventoryPatch{
		SKU:       42,
		Warehouse: 7,
	}

	sql, args := inventoryPatchStruct.Update("inventory", patch).Build()
	a.Equal(sql, "UPDATE inventory SET sku_id = ?, warehouse_id = ?, quantity = ?, reason = ?, note = ?")
	a.Equal(args, []interface{}{int64(42), int64(7), (*int64)(nil), (**string)(nil), (*string)(nil)})
}

func ExampleStruct_UpdatePartial() {
	type Stock struct {
		SKU      int64    `db:"sku_id"`
		Quantity *int64   `db:"quantity"`
		Reason   **string `db:"reason"`
	}

	var stockStruct = NewStruct(new(Stock))

	// The source cleared reason but never collected quantity.
	cleared := (*string)(nil)
	patch := &Stock{
		SKU:    42,
		Reason: &cleared,
	}

	ub, err := stockStruct.UpdatePartial("inventory", patch, "sku_id")
	if err != nil {
		// errors.Is(err, ErrUpdatePartialNoChanges) means there is nothing to write.
		panic(err)
	}

	sql, args := ub.Build()
	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// UPDATE inventory SET reason = ? WHERE sku_id = ?
	// [<nil> 42]
}

// TestStructUpdatePartialUsingExistingUpdateAPI locks in the pattern that only uses
// the pre-existing Struct#Update surface: pointer fields with `omitempty` express
// "not reached", **T expresses "explicitly NULL", and NumAssignment tells the caller
// whether anything is actually written.
func TestStructUpdatePartialUsingExistingUpdateAPI(t *testing.T) {
	a := assert.New(t)

	type stockPatch struct {
		SKU      int64    `db:"sku_id" fieldtag:"key"`
		Quantity *int64   `db:"quantity" fieldopt:"omitempty"`
		Reason   **string `db:"reason" fieldopt:"omitempty"`
		Note     *string  `db:"note" fieldopt:"omitempty"`
	}

	st := NewStruct(new(stockPatch))

	build := func(patch *stockPatch) *UpdateBuilder {
		ub := st.WithoutTag("key").Update("inventory", patch)
		ub.Where(ub.Equal("sku_id", patch.SKU))
		return ub
	}

	zero := int64(0)
	cleared := (*string)(nil)
	ub := build(&stockPatch{SKU: 42, Quantity: &zero, Reason: &cleared})

	sql, args := ub.Build()
	a.Equal(sql, "UPDATE inventory SET quantity = ?, reason = ? WHERE sku_id = ?")
	a.Equal(args, []interface{}{zero, nil, int64(42)})
	a.Equal(ub.NumAssignment(), 2)

	// Nothing was collected: the caller checks NumAssignment and skips the statement.
	// Building it would produce "UPDATE inventory WHERE sku_id = ?", which has no SET
	// and must never be executed, so it's the caller's job to stop here.
	ub = build(&stockPatch{SKU: 42})
	a.Equal(ub.NumAssignment(), 0)
}

func TestStructUpdatePartialTypedNilPointer(t *testing.T) {
	a := assert.New(t)

	var patch *inventoryPatch

	// A typed nil of the right struct type is an empty patch, not a type error.
	ub, err := inventoryPatchStruct.UpdatePartial("inventory", patch, "sku_id")
	a.Assert(errors.Is(err, ErrUpdatePartialNoChanges))
	a.Assert(ub == nil)
}

func TestStructUpdatePartialDriverValuerField(t *testing.T) {
	a := assert.New(t)

	type row struct {
		ID   int64           `db:"id"`
		Name *sql.NullString `db:"name"`
	}

	st := NewStruct(new(row))

	// A non-nil driver.Valuer is passed through so the database driver can encode it.
	// sql.NullString{Valid: false} is the idiomatic way to write an explicit NULL.
	null := sql.NullString{}
	ub, err := st.UpdatePartial("t", &row{ID: 1, Name: &null}, "id")
	a.NilError(err)

	sqlStr, args := ub.Build()
	a.Equal(sqlStr, "UPDATE t SET name = ? WHERE id = ?")
	a.Equal(args, []interface{}{&null, int64(1)})

	// A nil driver.Valuer pointer means the field wasn't reached.
	ub, err = st.UpdatePartial("t", &row{ID: 1}, "id")
	a.Assert(errors.Is(err, ErrUpdatePartialNoChanges))
	a.Assert(ub == nil)
}
