// Package resource contains the runtime contracts shared by generated
// Resource controllers and their injected business dependencies.
package resource

import (
	"context"
)

// HookSet defines the optional lifecycle callbacks for one Resource model.
//
// Before*/After* callbacks execute inside the controller transaction. Returning
// an error aborts that transaction; nil callbacks are skipped. AfterCommit*
// callbacks run only after a successful commit and receive value snapshots so
// they cannot retain or mutate the controller's model pointers.
type HookSet[M any] struct {
	BeforeCreate func(context.Context, Tx, *M) error
	AfterCreate  func(context.Context, Tx, *M) error

	BeforeUpdate func(context.Context, Tx, *M, *M, ColumnSet) error
	AfterUpdate  func(context.Context, Tx, *M, *M, ColumnSet) error

	BeforeDelete func(context.Context, Tx, *M) error
	AfterDelete  func(context.Context, Tx, *M) error

	BeforeBatchDelete func(context.Context, Tx, []*M) error
	AfterBatchDelete  func(context.Context, Tx, []*M) error

	AfterCommitCreate      func(context.Context, M)
	AfterCommitUpdate      func(context.Context, M)
	AfterCommitDelete      func(context.Context, M)
	AfterCommitBatchDelete func(context.Context, []M)
}
