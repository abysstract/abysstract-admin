package gen

import (
	"context"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var _ adminv1connect.DictServiceHandler = (*DictServiceAdapter)(nil)

type dictDataInjectedRepository struct {
	inner *DictDataRepository
}

func (d *dictDataInjectedRepository) Create(ctx context.Context, r resource.Session, m *model.SysDictDatum) error {

	return d.inner.Create(ctx, r, m)
}

func (d *dictDataInjectedRepository) GetByKey(ctx context.Context, r resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return d.inner.GetByKey(ctx, r, key, access)
}

func (d *dictDataInjectedRepository) GetByKeyForUpdate(ctx context.Context, r resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return d.inner.GetByKeyForUpdate(ctx, r, key, access)
}

func (d *dictDataInjectedRepository) Update(ctx context.Context, r resource.Session, key int64, m *model.SysDictDatum, columns repo.ColumnSet, access repo.AccessScope) error {

	return d.inner.Update(ctx, r, key, m, columns, access)
}

func (d *dictDataInjectedRepository) DeleteByKey(ctx context.Context, r resource.Session, key int64, access repo.AccessScope) error {

	return d.inner.DeleteByKey(ctx, r, key, access)
}

func (d *dictDataInjectedRepository) GetManyForUpdate(ctx context.Context, r resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysDictDatum, error) {

	return d.inner.GetManyForUpdate(ctx, r, keys, access)
}

func (d *dictDataInjectedRepository) BatchDelete(ctx context.Context, r resource.Session, keys []int64, access repo.AccessScope) (int64, error) {

	return d.inner.BatchDelete(ctx, r, keys, access)
}

func (d *dictDataInjectedRepository) List(ctx context.Context, r resource.Session, q DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {

	return d.inner.List(ctx, r, q, access)
}

var _ resource.Repository[model.SysDictDatum, DictDataListQuery] = (*dictDataInjectedRepository)(nil)
