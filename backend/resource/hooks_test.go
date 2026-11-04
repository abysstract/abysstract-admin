package resource

import (
	"context"
	"testing"
)

type hookSetTestModel struct {
	ID int64
}

func TestHookSetCompilesWithAllLifecycleCallbacks(t *testing.T) {
	ctx := context.Background()
	current := &hookSetTestModel{ID: 2}
	old := &hookSetTestModel{ID: 1}
	columns := NewColumnSet(Column("name"))

	hooks := HookSet[hookSetTestModel]{
		BeforeCreate: func(context.Context, Tx, *hookSetTestModel) error { return nil },
		AfterCreate:  func(context.Context, Tx, *hookSetTestModel) error { return nil },
		BeforeUpdate: func(context.Context, Tx, *hookSetTestModel, *hookSetTestModel, ColumnSet) error {
			return nil
		},
		AfterUpdate: func(context.Context, Tx, *hookSetTestModel, *hookSetTestModel, ColumnSet) error {
			return nil
		},
		BeforeDelete: func(context.Context, Tx, *hookSetTestModel) error { return nil },
		AfterDelete:  func(context.Context, Tx, *hookSetTestModel) error { return nil },
		BeforeBatchDelete: func(context.Context, Tx, []*hookSetTestModel) error {
			return nil
		},
		AfterBatchDelete: func(context.Context, Tx, []*hookSetTestModel) error {
			return nil
		},
		AfterCommitCreate:      func(context.Context, hookSetTestModel) {},
		AfterCommitUpdate:      func(context.Context, hookSetTestModel) {},
		AfterCommitDelete:      func(context.Context, hookSetTestModel) {},
		AfterCommitBatchDelete: func(context.Context, []hookSetTestModel) {},
	}

	if err := hooks.BeforeCreate(ctx, Tx{}, current); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if err := hooks.AfterCreate(ctx, Tx{}, current); err != nil {
		t.Fatalf("AfterCreate() error = %v", err)
	}
	if err := hooks.BeforeUpdate(ctx, Tx{}, old, current, columns); err != nil {
		t.Fatalf("BeforeUpdate() error = %v", err)
	}
	if err := hooks.AfterUpdate(ctx, Tx{}, old, current, columns); err != nil {
		t.Fatalf("AfterUpdate() error = %v", err)
	}
	if err := hooks.BeforeDelete(ctx, Tx{}, old); err != nil {
		t.Fatalf("BeforeDelete() error = %v", err)
	}
	if err := hooks.AfterDelete(ctx, Tx{}, old); err != nil {
		t.Fatalf("AfterDelete() error = %v", err)
	}
	if err := hooks.BeforeBatchDelete(ctx, Tx{}, []*hookSetTestModel{old, current}); err != nil {
		t.Fatalf("BeforeBatchDelete() error = %v", err)
	}
	if err := hooks.AfterBatchDelete(ctx, Tx{}, []*hookSetTestModel{old, current}); err != nil {
		t.Fatalf("AfterBatchDelete() error = %v", err)
	}
	hooks.AfterCommitCreate(ctx, *current)
	hooks.AfterCommitUpdate(ctx, *current)
	hooks.AfterCommitDelete(ctx, *old)
	hooks.AfterCommitBatchDelete(ctx, []hookSetTestModel{*old, *current})
}
