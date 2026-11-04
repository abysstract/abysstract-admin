package resource

import "context"

// ReadRepository is the minimal typed persistence contract for a partial
// Resource that has only migrated its standard List/Get read surface. It keeps
// password, relationship, status, and other write workflows outside the
// generated controller until they pass their own security gates.
type ReadRepository[M any, Q any] interface {
	GetByKey(context.Context, Session, int64, AccessScope) (*M, error)
	List(context.Context, Session, Q, AccessScope) ([]*M, int64, error)
}

// Repository is the complete typed persistence contract for one Resource.
// Q is the Resource-specific generated ListQuery type. Controllers depend
// only on this interface and pass authorization separately from ordinary
// query clauses.
type Repository[M any, Q any] interface {
	ReadRepository[M, Q]
	Create(context.Context, Session, *M) error
	GetByKeyForUpdate(context.Context, Session, int64, AccessScope) (*M, error)
	Update(context.Context, Session, int64, *M, ColumnSet, AccessScope) error
	DeleteByKey(context.Context, Session, int64, AccessScope) error
	GetManyForUpdate(context.Context, Session, []int64, AccessScope) ([]*M, error)
	BatchDelete(context.Context, Session, []int64, AccessScope) (int64, error)
}
