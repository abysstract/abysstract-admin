package gen

import (
	"context"
	"testing"

	"gorm.io/gorm"

	testutil "github.com/abysstract/abysstract-admin/backend/internal/testutil"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var _ resource.ReadRepository[model.SysUser, UserListQuery] = (*UserRepository)(nil)

type userCompileOverrides struct{ calls map[string]int }

func (o userCompileOverrides) GetUser(context.Context, *adminv1.GetUserRequest) (*adminv1.GetUserResponse, error) {
	if o.calls != nil {
		o.calls["GetUser"]++
	}
	return nil, nil
}
func (o userCompileOverrides) CreateUser(context.Context, *adminv1.CreateUserRequest) (*adminv1.CreateUserResponse, error) {
	if o.calls != nil {
		o.calls["CreateUser"]++
	}
	return nil, nil
}
func (o userCompileOverrides) UpdateUser(context.Context, *adminv1.UpdateUserRequest) (*adminv1.UpdateUserResponse, error) {
	if o.calls != nil {
		o.calls["UpdateUser"]++
	}
	return nil, nil
}
func (o userCompileOverrides) DeleteUser(context.Context, *adminv1.DeleteUserRequest) (*adminv1.DeleteUserResponse, error) {
	if o.calls != nil {
		o.calls["DeleteUser"]++
	}
	return nil, nil
}
func (o userCompileOverrides) BatchDeleteUsers(context.Context, *adminv1.BatchDeleteUsersRequest) (*adminv1.BatchDeleteUsersResponse, error) {
	if o.calls != nil {
		o.calls["BatchDeleteUsers"]++
	}
	return nil, nil
}
func (o userCompileOverrides) ChangeUserStatus(context.Context, *adminv1.ChangeUserStatusRequest) (*adminv1.ChangeUserStatusResponse, error) {
	if o.calls != nil {
		o.calls["ChangeUserStatus"]++
	}
	return nil, nil
}
func (o userCompileOverrides) ResetPassword(context.Context, *adminv1.ResetPasswordRequest) (*adminv1.ResetPasswordResponse, error) {
	if o.calls != nil {
		o.calls["ResetPassword"]++
	}
	return nil, nil
}

type userCompileScope struct{}

func (userCompileScope) DataScope(context.Context, string, string) repo.AccessScope {
	return repo.UnrestrictedAccessScope()
}

func TestGeneratedUserReadControllerAndWorkflowAdapterCompileGate(t *testing.T) {
	if _, err := NewUserController(UserControllerDeps{}); err == nil {
		t.Fatal("User controller accepted empty dependencies")
	}
	controller, err := NewUserController(UserControllerDeps{
		Store:             testutil.Store(t, repo.New(&gorm.DB{})),
		Repository:        NewUserRepository(),
		Overrides:         userCompileOverrides{},
		DataScopeProvider: userCompileScope{},
	})
	if err != nil {
		t.Fatalf("NewUserController: %v", err)
	}
	adapter, err := NewUserServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewUserServiceAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewUserServiceAdapter returned nil adapter")
	}
	var _ adminv1connect.UserServiceHandler = adapter
	var _ func(context.Context, *adminv1.ListUsersRequest) (*adminv1.ListUsersResponse, error) = adapter.ListUsers
	var _ func(context.Context, *adminv1.GetUserRequest) (*adminv1.GetUserResponse, error) = adapter.GetUser
}

func TestGeneratedUserSecurityWorkflowDelegationRunsExactlyOnce(t *testing.T) {
	calls := map[string]int{}
	controller, err := NewUserController(UserControllerDeps{
		Store:      testutil.Store(t, repo.New(&gorm.DB{})),
		Repository: NewUserRepository(), Overrides: userCompileOverrides{calls: calls}, DataScopeProvider: userCompileScope{},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewUserServiceAdapter(controller)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := adapter.GetUser(ctx, &adminv1.GetUserRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["GetUser"] != 1 {
		t.Fatalf("GetUser delegated %d times", calls["GetUser"])
	}
	if _, err := adapter.CreateUser(ctx, &adminv1.CreateUserRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["CreateUser"] != 1 {
		t.Fatalf("CreateUser delegated %d times", calls["CreateUser"])
	}
	if _, err := adapter.UpdateUser(ctx, &adminv1.UpdateUserRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["UpdateUser"] != 1 {
		t.Fatalf("UpdateUser delegated %d times", calls["UpdateUser"])
	}
	if _, err := adapter.DeleteUser(ctx, &adminv1.DeleteUserRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["DeleteUser"] != 1 {
		t.Fatalf("DeleteUser delegated %d times", calls["DeleteUser"])
	}
	if _, err := adapter.BatchDeleteUsers(ctx, &adminv1.BatchDeleteUsersRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["BatchDeleteUsers"] != 1 {
		t.Fatalf("BatchDeleteUsers delegated %d times", calls["BatchDeleteUsers"])
	}
	if _, err := adapter.ChangeUserStatus(ctx, &adminv1.ChangeUserStatusRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["ChangeUserStatus"] != 1 {
		t.Fatalf("ChangeUserStatus delegated %d times", calls["ChangeUserStatus"])
	}
	if _, err := adapter.ResetPassword(ctx, &adminv1.ResetPasswordRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls["ResetPassword"] != 1 {
		t.Fatalf("ResetPassword delegated %d times", calls["ResetPassword"])
	}
}
