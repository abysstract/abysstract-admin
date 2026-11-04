//go:build integration

package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func TestRepoInTransactionCommitsAndRollsBack(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	r := New(gdb)
	committed := &model.SysDictType{DictName: "事务提交", DictType: "transaction_commit"}
	if err := r.InTransaction(ctx, func(tx *Repo) error {
		if tx == r || tx.db == r.db {
			t.Fatal("InTransaction must bind the callback to a child Repo with the transaction DB")
		}
		return resourcegen.NewDictTypeRepository().Create(ctx, tx, committed)
	}); err != nil {
		t.Fatalf("InTransaction(commit): %v", err)
	}
	if _, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, r, committed.ID, UnrestrictedAccessScope()); err != nil {
		t.Fatalf("committed record missing: %v", err)
	}

	rolledBack := &model.SysDictType{DictName: "事务回滚", DictType: "transaction_rollback"}
	rollbackErr := errors.New("force rollback")
	if err := r.InTransaction(ctx, func(tx *Repo) error {
		if err := resourcegen.NewDictTypeRepository().Create(ctx, tx, rolledBack); err != nil {
			return err
		}
		return rollbackErr
	}); !errors.Is(err, rollbackErr) {
		t.Fatalf("InTransaction(rollback) error = %v, want %v", err, rollbackErr)
	}
	if _, err := resourcegen.NewDictTypeRepository().GetByKey(ctx, r, rolledBack.ID, UnrestrictedAccessScope()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rolled-back record lookup error = %v, want ErrNotFound", err)
	}
}
