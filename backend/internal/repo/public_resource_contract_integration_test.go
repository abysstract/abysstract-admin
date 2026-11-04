//go:build integration

package repo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestPublicAndInternalGenericDatabaseParity(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	scope := resource.UnrestrictedAccessScope()
	for _, public := range []bool{false, true} {
		t.Run(fmt.Sprintf("public=%v", public), func(t *testing.T) {
			row := &model.SysDictType{DictName: "initial", DictType: fmt.Sprintf("parity_%v", public), Status: 1}
			sentinel := errors.New("rollback both entry points")
			create := func(tx resource.Tx) error {
				if public {
					return resourcegen.NewDictTypeRepository().Create(ctx, tx, row)
				}
				return resourcegen.NewDictTypeRepository().Create(ctx, New(tx.DB()), row)
			}
			if err := store.InTransaction(ctx, func(tx resource.Tx) error {
				if err := create(tx); err != nil {
					return err
				}
				internal, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, New(tx.DB()), row.ID, scope)
				if err != nil {
					return err
				}
				external, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, tx, row.ID, scope)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(internal, external) {
					t.Fatalf("read mismatch: %+v %+v", internal, external)
				}
				return sentinel
			}); !errors.Is(err, sentinel) {
				t.Fatalf("rollback: %v", err)
			}
			if _, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, store, row.ID, scope); !errors.Is(err, ErrNotFound) {
				t.Fatalf("rolled back lookup: %v", err)
			}
			row.ID = 0
			if err := store.InTransaction(ctx, create); err != nil {
				t.Fatal(err)
			}
			if row.ID == 0 || row.CreateTime.IsZero() {
				t.Fatalf("RETURNING did not populate fields: %+v", row)
			}
			if err := store.InTransaction(ctx, func(tx resource.Tx) error {
				internal := New(tx.DB())
				before, err := resourcegen.NewDictTypeRepository().GetByKeyForUpdate(ctx, tx, row.ID, scope)
				if err != nil {
					return err
				}
				locked, err := resourcegen.NewDictTypeRepository().GetByKeyForUpdate(ctx, internal, row.ID, scope)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(before, locked) {
					t.Fatal("lock read mismatch")
				}
				a, err := resourcegen.NewDictTypeRepository().GetManyForUpdate(ctx, tx, []int64{row.ID}, scope)
				if err != nil {
					return err
				}
				b, err := resourcegen.NewDictTypeRepository().GetManyForUpdate(ctx, internal, []int64{row.ID}, scope)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(a, b) {
					t.Fatal("batch lock mismatch")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := store.InTransaction(ctx, func(tx resource.Tx) error {
				if err := resourcegen.NewDictTypeRepository().DeleteByKey(ctx, tx, row.ID, resource.AccessScope{}); !errors.Is(err, ErrNotFound) {
					t.Fatalf("denied delete: %v", err)
				}
				var affected int64
				var err error
				if public {
					affected, err = resourcegen.NewDictTypeRepository().BatchDelete(ctx, tx, []int64{row.ID}, scope)
				} else {
					affected, err = resourcegen.NewDictTypeRepository().BatchDelete(ctx, New(tx.DB()), []int64{row.ID}, scope)
				}
				if err == nil && affected != 1 {
					t.Fatalf("affected=%d", affected)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, store, row.ID, scope); !errors.Is(err, ErrNotFound) {
				t.Fatalf("deleted lookup: %v", err)
			}
		})
	}
}
