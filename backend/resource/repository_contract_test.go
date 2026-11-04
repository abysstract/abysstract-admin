package resource

import (
	"context"
	"testing"
)

type repositoryContractModel struct{}
type repositoryContractQuery struct{}

type repositoryContractImplementation struct{}

func (*repositoryContractImplementation) Create(context.Context, Session, *repositoryContractModel) error {
	return nil
}
func (*repositoryContractImplementation) GetByKey(context.Context, Session, int64, AccessScope) (*repositoryContractModel, error) {
	return nil, nil
}
func (*repositoryContractImplementation) GetByKeyForUpdate(context.Context, Session, int64, AccessScope) (*repositoryContractModel, error) {
	return nil, nil
}
func (*repositoryContractImplementation) Update(context.Context, Session, int64, *repositoryContractModel, ColumnSet, AccessScope) error {
	return nil
}
func (*repositoryContractImplementation) DeleteByKey(context.Context, Session, int64, AccessScope) error {
	return nil
}
func (*repositoryContractImplementation) GetManyForUpdate(context.Context, Session, []int64, AccessScope) ([]*repositoryContractModel, error) {
	return nil, nil
}
func (*repositoryContractImplementation) BatchDelete(context.Context, Session, []int64, AccessScope) (int64, error) {
	return 0, nil
}
func (*repositoryContractImplementation) List(context.Context, Session, repositoryContractQuery, AccessScope) ([]*repositoryContractModel, int64, error) {
	return nil, 0, nil
}

var _ Repository[repositoryContractModel, repositoryContractQuery] = (*repositoryContractImplementation)(nil)

// A custom persistence implementation/decorator stays fully typed: it embeds
// the complete Repository[M,Q] contract rather than using a repo_method_map or
// runtime any assertion for individual methods.
func TestTypedRepositoryDecoratorCompiles(t *testing.T) {
	inner := &repositoryContractImplementation{}
	decorator := &struct {
		Repository[repositoryContractModel, repositoryContractQuery]
	}{Repository: inner}

	var typed Repository[repositoryContractModel, repositoryContractQuery] = decorator
	if typed == nil {
		t.Fatal("typed decorator must retain a non-nil complete Repository")
	}
}
