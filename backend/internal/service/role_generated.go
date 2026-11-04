package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// roleRepository is the complete typed persistence boundary for the standard
// Role CRUD. It deliberately delegates scalar conversion/audit handling to the
// generated adapter and keeps soft-delete/relation cleanup in repo.
type roleRepository struct {
	inner *resourcegen.RoleRepository
}

var _ resource.Repository[model.SysRole, resourcegen.RoleListQuery] = (*roleRepository)(nil)

func newRoleRepository() *roleRepository {
	return &roleRepository{inner: resourcegen.NewRoleRepository()}
}

func (r *roleRepository) Create(ctx context.Context, tx resource.Session, m *model.SysRole) error {

	return r.inner.Create(ctx, tx, m)
}

func (r *roleRepository) GetByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysRole, error) {
	tx := repo.New(txSession.DB())

	return tx.GetRoleResource(ctx, key, access)
}

func (r *roleRepository) GetByKeyForUpdate(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysRole, error) {
	tx := repo.New(txSession.DB())

	return tx.GetRoleResourceForUpdate(ctx, key, access)
}

func (r *roleRepository) Update(ctx context.Context, txSession resource.Session, key int64, m *model.SysRole, columns repo.ColumnSet, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	if _, err := tx.GetRoleResourceForUpdate(ctx, key, access); err != nil {
		return err
	}
	return r.inner.Update(ctx, tx, key, m, columns, access)
}

func (r *roleRepository) DeleteByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) error {
	tx := repo.New(txSession.DB())

	return tx.DeleteRoleResource(ctx, key, access)
}

func (r *roleRepository) GetManyForUpdate(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysRole, error) {
	tx := repo.New(txSession.DB())

	return tx.GetManyRoleResourcesForUpdate(ctx, keys, access)
}

func (r *roleRepository) BatchDelete(ctx context.Context, txSession resource.Session, keys []int64, access repo.AccessScope) (int64, error) {
	tx := repo.New(txSession.DB())

	return tx.BatchDeleteRoleResources(ctx, keys, access)
}

func (r *roleRepository) List(ctx context.Context, txSession resource.Session, q resourcegen.RoleListQuery, access repo.AccessScope) ([]*model.SysRole, int64, error) {
	tx := repo.New(txSession.DB())

	return tx.ListRoleRows(ctx, repo.Page{Num: q.PageNum, Size: q.PageSize}, q.Keyword, q.Status, q.Orders, access)
}

func newRoleHooks() *resource.HookSet[model.SysRole] {
	return &resource.HookSet[model.SysRole]{
		BeforeCreate: func(_ context.Context, txSession resource.Tx, role *model.SysRole) error {
			tx := repo.New(txSession.DB())

			if err := tx.PrepareRoleCreate(role); err != nil {
				return err
			}
			if err := repo.ValidateRoleDataScope(role.DataScope); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
			}
			return nil
		},
		BeforeUpdate: func(_ context.Context, _ resource.Tx, _ *model.SysRole, role *model.SysRole, _ repo.ColumnSet) error {
			if err := repo.ValidateRoleDataScope(role.DataScope); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
			}
			return nil
		},
		BeforeDelete: func(_ context.Context, _ resource.Tx, role *model.SysRole) error {
			if err := repo.ValidateRoleDelete(role); err != nil {
				return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
			}
			return nil
		},
		BeforeBatchDelete: func(_ context.Context, _ resource.Tx, roles []*model.SysRole) error {
			for _, role := range roles {
				if err := repo.ValidateRoleDelete(role); err != nil {
					return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
				}
			}
			return nil
		},
	}
}

type roleOverrides struct {
	repo *repo.Repo
}

var _ resourcegen.RoleOverrides = (*roleOverrides)(nil)

func (o *roleOverrides) ChangeRoleStatus(ctx context.Context, req *adminv1.ChangeRoleStatusRequest) (*adminv1.ChangeRoleStatusResponse, error) {
	status, err := roleStatusValue(req.GetStatus())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	if err := o.repo.ChangeRoleStatusResource(ctx, req.GetId(), status, repo.UnrestrictedAccessScope()); err != nil {
		return nil, roleCustomError(err)
	}
	return &adminv1.ChangeRoleStatusResponse{}, nil
}

func (o *roleOverrides) GetRoleMenus(ctx context.Context, req *adminv1.GetRoleMenusRequest) (*adminv1.GetRoleMenusResponse, error) {
	if _, err := o.repo.GetRoleResource(ctx, req.GetRoleId(), repo.UnrestrictedAccessScope()); err != nil {
		return nil, roleCustomError(err)
	}
	menuIDs, err := o.repo.MenuIDsOfRole(ctx, req.GetRoleId())
	if err != nil {
		return nil, roleCustomError(err)
	}
	return &adminv1.GetRoleMenusResponse{MenuIds: menuIDs}, nil
}

func (o *roleOverrides) UpdateRoleMenus(ctx context.Context, req *adminv1.UpdateRoleMenusRequest) (*adminv1.UpdateRoleMenusResponse, error) {
	err := o.repo.InTransaction(ctx, func(tx *repo.Repo) error {
		return tx.UpdateRoleMenusResource(ctx, req.GetRoleId(), req.GetMenuIds(), repo.UnrestrictedAccessScope())
	})
	if err != nil {
		return nil, roleCustomError(err)
	}
	return &adminv1.UpdateRoleMenusResponse{}, nil
}

func roleStatusValue(status adminv1.Status) (int16, error) {
	switch status {
	case adminv1.Status_STATUS_ENABLED:
		return 1, nil
	case adminv1.Status_STATUS_DISABLED:
		return 2, nil
	default:
		return 0, fmt.Errorf("未知枚举值 %v", status)
	}
}

func roleCustomError(err error) error {
	if code := connect.CodeOf(err); code != connect.CodeUnknown {
		return err
	}
	return toConnectError(err)
}

// NewGeneratedRoleService composes the generated standard six-RPC controller
// with the typed relation/status overrides. No legacy standard CRUD handler is
// retained beside this adapter.
func NewGeneratedRoleService(baseRepo *repo.Repo) (adminv1connect.RoleServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	controller, err := resourcegen.NewRoleController(resourcegen.RoleControllerDeps{
		Store:      store,
		Hooks:      resource.ComposeHooks(newRoleHooks(), nil),
		Repository: newRoleRepository(),
		Overrides:  &roleOverrides{repo: baseRepo},
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewRoleServiceAdapter(controller)
}
