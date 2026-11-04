package resource

import (
	"context"
	"errors"
)

// DataScopeProvider resolves the fail-closed row-level authorization scope
// for one resource. Build passes the validated dept/creator DB column names;
// implementations must return a deny-all scope when the caller has no visible
// rows. Controllers treat the result as opaque.
type DataScopeProvider interface {
	DataScope(ctx context.Context, deptDBName, creatorDBName string) AccessScope
}

// ErrScopeDenied is returned by CreateScopeValidator when the model being
// created would fall outside the caller's data scope. Controllers map it to
// PermissionDenied; any other error maps to Internal.
var ErrScopeDenied = errors.New("resource: create scope denied")

// CreateScopeValidator checks, inside the create transaction and after
// BeforeCreate, whether the about-to-be-created model is within the caller's
// data scope. It returns ErrScopeDenied for scope violations and any other
// error for internal failures.
type CreateScopeValidator[M any] func(ctx context.Context, txRepo Tx, m *M) error
