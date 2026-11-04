package resource

import (
	"context"
	"log/slog"
	"slices"
)

// ComposeHooks snapshots both sets of callbacks. Before runs system then
// business; After runs business then system. Each committed callback has an
// independent panic boundary, and batch callbacks own separate slice copies.
func ComposeHooks[M any](system, business *HookSet[M]) *HookSet[M] {
	var a, b HookSet[M]
	if system != nil {
		a = *system
	}
	if business != nil {
		b = *business
	}
	return &HookSet[M]{
		BeforeCreate:           chainModel(a.BeforeCreate, b.BeforeCreate),
		AfterCreate:            chainModel(b.AfterCreate, a.AfterCreate),
		BeforeUpdate:           chainUpdate(a.BeforeUpdate, b.BeforeUpdate),
		AfterUpdate:            chainUpdate(b.AfterUpdate, a.AfterUpdate),
		BeforeDelete:           chainModel(a.BeforeDelete, b.BeforeDelete),
		AfterDelete:            chainModel(b.AfterDelete, a.AfterDelete),
		BeforeBatchDelete:      chainBatch(a.BeforeBatchDelete, b.BeforeBatchDelete),
		AfterBatchDelete:       chainBatch(b.AfterBatchDelete, a.AfterBatchDelete),
		AfterCommitCreate:      commitPair("AfterCommitCreate", a.AfterCommitCreate, b.AfterCommitCreate, nil),
		AfterCommitUpdate:      commitPair("AfterCommitUpdate", a.AfterCommitUpdate, b.AfterCommitUpdate, nil),
		AfterCommitDelete:      commitPair("AfterCommitDelete", a.AfterCommitDelete, b.AfterCommitDelete, nil),
		AfterCommitBatchDelete: commitPair("AfterCommitBatchDelete", a.AfterCommitBatchDelete, b.AfterCommitBatchDelete, func(ms []M) []M { return slices.Clone(ms) }),
	}
}

func chainModel[M any](first, second func(context.Context, Tx, *M) error) func(context.Context, Tx, *M) error {
	if first == nil && second == nil {
		return nil
	}
	return func(ctx context.Context, tx Tx, m *M) error {
		if first != nil {
			if err := first(ctx, tx, m); err != nil {
				return err
			}
		}
		if second != nil {
			return second(ctx, tx, m)
		}
		return nil
	}
}

func chainUpdate[M any](first, second func(context.Context, Tx, *M, *M, ColumnSet) error) func(context.Context, Tx, *M, *M, ColumnSet) error {
	if first == nil && second == nil {
		return nil
	}
	return func(ctx context.Context, tx Tx, old, m *M, columns ColumnSet) error {
		if first != nil {
			if err := first(ctx, tx, old, m, columns); err != nil {
				return err
			}
		}
		if second != nil {
			return second(ctx, tx, old, m, columns)
		}
		return nil
	}
}

func chainBatch[M any](first, second func(context.Context, Tx, []*M) error) func(context.Context, Tx, []*M) error {
	if first == nil && second == nil {
		return nil
	}
	return func(ctx context.Context, tx Tx, models []*M) error {
		if first != nil {
			if err := first(ctx, tx, models); err != nil {
				return err
			}
		}
		if second != nil {
			return second(ctx, tx, models)
		}
		return nil
	}
}

func commitPair[M any](hook string, system, business func(context.Context, M), clone func(M) M) func(context.Context, M) {
	if system == nil && business == nil {
		return nil
	}
	return func(ctx context.Context, snapshot M) {
		first, second := snapshot, snapshot
		if clone != nil {
			first, second = clone(snapshot), clone(snapshot)
		}
		if system != nil {
			runCommittedHook(ctx, hook, "system", func() { system(ctx, first) })
		}
		if business != nil {
			runCommittedHook(ctx, hook, "business", func() { business(ctx, second) })
		}
	}
}

func runCommittedHook(ctx context.Context, hook, source string, fn func()) {
	defer func() {
		if p := recover(); p != nil {
			slog.ErrorContext(ctx, "resource after-commit hook panicked", "hook", hook, "source", source, "panic", p)
		}
	}()
	fn()
}
