package service

import (
	"context"

	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

const deptDefaultStatus int16 = 1

// deptRepository is a complete typed injected Repository. It delegates the
// ordinary create/update conversion mechanics to the generated adapter while
// keeping Dept's published soft-delete and active-row/ancestor semantics in
// the repository package.
type deptRepository struct {
	inner *resourcegen.DeptRepository
}

var _ resource.Repository[model.SysDept, resourcegen.DeptListQuery] = (*deptRepository)(nil)

func newDeptRepository() *deptRepository {
	return &deptRepository{inner: resourcegen.NewDeptRepository()}
}

func (r *deptRepository) Create(ctx context.Context, tx resource.Session, m *model.SysDept) error {

	return r.inner.Create(ctx, tx, m)
}

func (r *deptRepository) GetByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysDept, error) {
	tx := repo.New(txSession.DB())

	return tx.GetDeptResource(ctx, key, access)
}

func (r *deptRepository) GetByKeyForUpdate(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysDept, error) {
	tx := repo.New(txSession.DB())

	return tx.GetDeptResourceForUpdate(ctx, key, access)
}

func (r *deptRepository) Update(ctx context.Context, txSession resource.Session, key int64, m *model.SysDept, columns repo.ColumnSet, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	if _, err := tx.GetDeptResourceForUpdate(ctx, key, access); err != nil {
		return err
	}
	return r.inner.Update(ctx, tx, key, m, columns, access)
}

func (r *deptRepository) DeleteByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	return tx.SoftDeleteDept(ctx, key, access)
}

func (r *deptRepository) GetManyForUpdate(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysDept, error) {
	tx := repo.New(txSession.DB())

	return tx.GetManyDeptResourcesForUpdate(ctx, keys, access)
}

func (r *deptRepository) BatchDelete(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) (int64, error) {
	tx := repo.New(txSession.DB())

	return tx.SoftBatchDeleteDepts(ctx, keys, access)
}

func (r *deptRepository) List(ctx context.Context, txSession resource.Session, q resourcegen.DeptListQuery, access repo.AccessScope) ([]*model.SysDept, int64, error) {
	tx := repo.New(txSession.DB())

	return tx.ListDeptRows(ctx, repo.Page{Num: q.PageNum, Size: q.PageSize}, q.Keyword, q.ParentId, q.Status, q.Orders, access)
}

func validateDeptModel(d *model.SysDept) error {
	return newValidator().
		maxLen("负责人", d.Leader, 64).
		maxLen("手机号", d.Phone, 32).
		maxLen("邮箱", d.Email, 128).
		email("邮箱", d.Email).
		err()
}

// newDeptHooks contains only Dept-specific semantics. Standard required and
// maxLen checks remain generated from schema/dept.go; email format, status/
// ancestors defaults, parent-cycle checks and child protection stay here.
func newDeptHooks() *resource.HookSet[model.SysDept] {
	return &resource.HookSet[model.SysDept]{
		BeforeCreate: func(ctx context.Context, txSession resource.Tx, d *model.SysDept) error {
			tx := repo.New(txSession.DB())

			if err := validateDeptModel(d); err != nil {
				return err
			}
			if err := tx.PrepareDeptCreate(ctx, d); err != nil {
				return err
			}
			if d.Status == 0 {
				d.Status = deptDefaultStatus
			}
			return nil
		},
		BeforeUpdate: func(ctx context.Context, txSession resource.Tx, _ *model.SysDept, d *model.SysDept, columns repo.ColumnSet) error {
			tx := repo.New(txSession.DB())

			if err := validateDeptModel(d); err != nil {
				return err
			}
			if columns.Contains(resourcegen.DeptColumnParentId) {
				return tx.MoveDeptSubtree(ctx, d.ID, d.ParentID)
			}
			return nil
		},
		BeforeDelete: func(ctx context.Context, txSession resource.Tx, d *model.SysDept) error {
			tx := repo.New(txSession.DB())

			return tx.ValidateDeptDelete(ctx, d.ID)
		},
		BeforeBatchDelete: func(ctx context.Context, txSession resource.Tx, list []*model.SysDept) error {
			tx := repo.New(txSession.DB())

			for _, d := range list {
				if err := tx.ValidateDeptDelete(ctx, d.ID); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// NewGeneratedDeptService composes the generated standard CRUD Controller with
// Dept hooks and the injected soft-delete Repository.
func NewGeneratedDeptService(baseRepo *repo.Repo) (adminv1connect.DeptServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	controller, err := resourcegen.NewDeptController(resourcegen.DeptControllerDeps{
		Store:      store,
		Hooks:      resource.ComposeHooks(newDeptHooks(), nil),
		Repository: newDeptRepository(),
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewDeptServiceAdapter(controller)
}
