package service

import (
	"context"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// userReadRepository is the only generated Resource persistence boundary used
// by the first User phase List path. It applies active-row and DataScope predicates while
// leaving password, role binding, status, and delete workflows in UserService.
type userReadRepository struct{}

var _ resource.ReadRepository[model.SysUser, resourcegen.UserListQuery] = (*userReadRepository)(nil)

func newUserReadRepository() *userReadRepository {
	return &userReadRepository{}
}

func (r *userReadRepository) GetByKey(ctx context.Context, txSession resource.Session, key int64, access repo.AccessScope) (*model.SysUser, error) {
	tx := repo.New(txSession.DB())

	return tx.GetUserResource(ctx, key, access)
}

func (r *userReadRepository) List(ctx context.Context, txSession resource.Session, query resourcegen.UserListQuery, access repo.AccessScope) ([]*model.SysUser, int64, error) {
	tx := repo.New(txSession.DB())

	return tx.ListUserRows(ctx, repo.Page{Num: query.PageNum, Size: query.PageSize}, query.Keyword, query.Status, query.DeptId, query.Orders, access)
}

type userDataScopeProvider struct {
	repo *repo.Repo
}

var _ resource.DataScopeProvider = (*userDataScopeProvider)(nil)

func (p *userDataScopeProvider) DataScope(ctx context.Context, deptDBName, creatorDBName string) repo.AccessScope {
	return p.repo.DataScopeAccessScope(ctx, deptDBName, creatorDBName)
}

// userOverrides keeps the security-sensitive User workflows behind the
// generated Partial controller's exact Connect signatures. No any assertion or
// second registered User handler is used.
type userOverrides struct {
	legacy *UserService
}

var _ resourcegen.UserOverrides = (*userOverrides)(nil)

func (o *userOverrides) GetUser(ctx context.Context, req *adminv1.GetUserRequest) (*adminv1.GetUserResponse, error) {
	return o.legacy.GetUser(ctx, req)
}

func (o *userOverrides) CreateUser(ctx context.Context, req *adminv1.CreateUserRequest) (*adminv1.CreateUserResponse, error) {
	return o.legacy.CreateUser(ctx, req)
}

func (o *userOverrides) UpdateUser(ctx context.Context, req *adminv1.UpdateUserRequest) (*adminv1.UpdateUserResponse, error) {
	return o.legacy.UpdateUser(ctx, req)
}

func (o *userOverrides) DeleteUser(ctx context.Context, req *adminv1.DeleteUserRequest) (*adminv1.DeleteUserResponse, error) {
	return o.legacy.DeleteUser(ctx, req)
}

func (o *userOverrides) BatchDeleteUsers(ctx context.Context, req *adminv1.BatchDeleteUsersRequest) (*adminv1.BatchDeleteUsersResponse, error) {
	return o.legacy.BatchDeleteUsers(ctx, req)
}

func (o *userOverrides) ChangeUserStatus(ctx context.Context, req *adminv1.ChangeUserStatusRequest) (*adminv1.ChangeUserStatusResponse, error) {
	return o.legacy.ChangeUserStatus(ctx, req)
}

func (o *userOverrides) ResetPassword(ctx context.Context, req *adminv1.ResetPasswordRequest) (*adminv1.ResetPasswordResponse, error) {
	return o.legacy.ResetPassword(ctx, req)
}

// NewGeneratedUserService composes generated User List with the existing
// typed security workflows. The optional refresh repository keeps
// ResetPassword session revocation semantics while remaining outside generated
// CRUD. This is deliberately Partial: ordinary write workflows are not
// claimed complete until their audit gates pass in a later step.
func NewGeneratedUserService(baseRepo *repo.Repo, refreshRepos ...*repo.RefreshTokenRepo) (adminv1connect.UserServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	legacy := NewUserService(baseRepo)
	if len(refreshRepos) > 0 && refreshRepos[0] != nil {
		legacy = legacy.WithRefreshTokenRepo(refreshRepos[0])
	}
	controller, err := resourcegen.NewUserController(resourcegen.UserControllerDeps{
		Store:             store,
		Repository:        newUserReadRepository(),
		Overrides:         &userOverrides{legacy: legacy},
		DataScopeProvider: &userDataScopeProvider{repo: baseRepo},
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewUserServiceAdapter(controller)
}
