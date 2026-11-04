package gen

import (
	"context"
	"errors"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func seedDictType(t *testing.T, a *DictTypeRepository, r *repo.Repo, name, dtype string) *model.SysDictType {
	t.Helper()
	m := &model.SysDictType{DictName: name, DictType: dtype, Status: 1}
	if err := a.Create(context.Background(), r, m); err != nil {
		t.Fatalf("create: %v", err)
	}
	return m
}

var errRollbackForTest = errors.New("rollback for test")

// injectedDictTypeRepository is a typed alternate Repository implementation.
// Embedding the full interface is deliberate: it cannot silently omit a new
// protected operation or use runtime method discovery.
type injectedDictTypeRepository struct {
	resource.Repository[model.SysDictType, DictTypeListQuery]
}
