//go:build integration

package repo

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// Database errors must win over RowsAffected == 0. Otherwise a failed UPDATE
// or DELETE would be misreported as NotFound and hide operational failures.
func TestGenericWritesReturnDatabaseErrorBeforeRowsAffected(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := New(gdb)
	ctx := context.Background()
	row := &model.SysDictType{DictName: "原值", DictType: "generic_error_order", Status: 1}
	if err := resourcegen.NewDictTypeRepository().Create(ctx, r, row); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("delete", func(t *testing.T) {
		injected := errors.New("injected delete failure")
		const callback = "resource:test:generic-delete-error"
		if err := gdb.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
			tx.AddError(injected)
		}); err != nil {
			t.Fatalf("register delete callback: %v", err)
		}
		t.Cleanup(func() { _ = gdb.Callback().Delete().Remove(callback) })

		err := resourcegen.NewDictTypeRepository().DeleteByKey(ctx, r, row.ID, UnrestrictedAccessScope())
		if !errors.Is(err, injected) {
			t.Fatalf("DeleteByKey() error = %v, want injected database error", err)
		}
		if errors.Is(err, ErrNotFound) {
			t.Fatalf("DeleteByKey() must not mask database error as ErrNotFound: %v", err)
		}
	})

	t.Run("batch delete", func(t *testing.T) {
		injected := errors.New("injected batch delete failure")
		const callback = "resource:test:generic-batch-delete-error"
		if err := gdb.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
			tx.AddError(injected)
		}); err != nil {
			t.Fatalf("register batch delete callback: %v", err)
		}
		t.Cleanup(func() { _ = gdb.Callback().Delete().Remove(callback) })

		_, err := resourcegen.NewDictTypeRepository().BatchDelete(ctx, r, []int64{row.ID}, UnrestrictedAccessScope())
		if !errors.Is(err, injected) {
			t.Fatalf("BatchDelete() error = %v, want injected database error", err)
		}
		if errors.Is(err, ErrNotFound) {
			t.Fatalf("BatchDelete() must not mask database error as ErrNotFound: %v", err)
		}
	})
}
