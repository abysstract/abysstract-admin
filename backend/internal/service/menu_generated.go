package service

import (
	"context"

	"connectrpc.com/connect/v2"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// menuRepository is the complete typed injected persistence boundary. The
// generated adapter remains the source for ordinary CRUD column/audit logic;
// this decorator adds role_menu cleanup and parent/child persistence rules.
type menuRepository struct {
	inner *resourcegen.MenuRepository
}

var _ resource.Repository[model.SysMenu, resourcegen.MenuListQuery] = (*menuRepository)(nil)

func newMenuRepository() *menuRepository {
	return &menuRepository{inner: resourcegen.NewMenuRepository()}
}

func (r *menuRepository) Create(ctx context.Context, tx resource.Session, m *model.SysMenu) error {

	return r.inner.Create(ctx, tx, m)
}

func (r *menuRepository) GetByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysMenu, error) {
	tx := repo.New(txSession.DB())

	return tx.GetMenuResource(ctx, key, access)
}

func (r *menuRepository) GetByKeyForUpdate(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysMenu, error) {
	tx := repo.New(txSession.DB())

	return tx.GetMenuResourceForUpdate(ctx, key, access)
}

func (r *menuRepository) Update(ctx context.Context, txSession resource.Session, key int64, m *model.SysMenu, columns repo.ColumnSet, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	if _, err := tx.GetMenuResourceForUpdate(ctx, key, access); err != nil {
		return err
	}
	return r.inner.Update(ctx, tx, key, m, columns, access)
}

func (r *menuRepository) DeleteByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	return tx.DeleteMenuResource(ctx, key, access)
}

func (r *menuRepository) GetManyForUpdate(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysMenu, error) {
	tx := repo.New(txSession.DB())

	return tx.GetManyMenuResourcesForUpdate(ctx, keys, access)
}

func (r *menuRepository) BatchDelete(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) (int64, error) {
	tx := repo.New(txSession.DB())

	return tx.BatchDeleteMenuResources(ctx, keys, access)
}

func (r *menuRepository) List(ctx context.Context, txSession resource.Session, q resourcegen.MenuListQuery, access repo.AccessScope) ([]*model.SysMenu, int64, error) {
	tx := repo.New(txSession.DB())

	return tx.ListMenuRows(ctx, repo.Page{Num: q.PageNum, Size: q.PageSize}, q.Keyword, q.ParentId, q.MenuType, q.Status, q.Orders, access)
}

func newMenuHooks() *resource.HookSet[model.SysMenu] {
	return &resource.HookSet[model.SysMenu]{
		BeforeCreate: func(ctx context.Context, txSession resource.Tx, m *model.SysMenu) error {
			tx := repo.New(txSession.DB())

			if err := repo.ValidateMenuType(m.MenuType); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
			}
			return tx.PrepareMenuCreate(ctx, m)
		},
		BeforeUpdate: func(ctx context.Context, txSession resource.Tx, _ *model.SysMenu, m *model.SysMenu, columns repo.ColumnSet) error {
			tx := repo.New(txSession.DB())

			if err := repo.ValidateMenuType(m.MenuType); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
			}
			if columns.Contains(resourcegen.MenuColumnParentId) {
				return tx.ValidateMenuParent(ctx, m.ID, m.ParentID)
			}
			return nil
		},
		BeforeDelete: func(ctx context.Context, txSession resource.Tx, m *model.SysMenu) error {
			tx := repo.New(txSession.DB())

			return tx.ValidateMenuDelete(ctx, m.ID)
		},
		BeforeBatchDelete: func(ctx context.Context, txSession resource.Tx, list []*model.SysMenu) error {
			tx := repo.New(txSession.DB())

			for _, m := range list {
				if err := tx.ValidateMenuDelete(ctx, m.ID); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// NewGeneratedMenuService composes the generated standard Menu Controller with
// the typed Repository and the small set of Menu-specific Hooks.
func NewGeneratedMenuService(baseRepo *repo.Repo) (adminv1connect.MenuServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	controller, err := resourcegen.NewMenuController(resourcegen.MenuControllerDeps{
		Store:      store,
		Hooks:      resource.ComposeHooks(newMenuHooks(), nil),
		Repository: newMenuRepository(),
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewMenuServiceAdapter(controller)
}
