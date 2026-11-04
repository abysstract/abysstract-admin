package gen

import (
	"context"
	"testing"

	"gorm.io/gorm"

	testutil "github.com/abysstract/abysstract-admin/backend/internal/testutil"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

type roleCompileOverrides struct{}

func (roleCompileOverrides) ChangeRoleStatus(context.Context, *adminv1.ChangeRoleStatusRequest) (*adminv1.ChangeRoleStatusResponse, error) {
	return nil, nil
}
func (roleCompileOverrides) GetRoleMenus(context.Context, *adminv1.GetRoleMenusRequest) (*adminv1.GetRoleMenusResponse, error) {
	return nil, nil
}
func (roleCompileOverrides) UpdateRoleMenus(context.Context, *adminv1.UpdateRoleMenusRequest) (*adminv1.UpdateRoleMenusResponse, error) {
	return nil, nil
}

func TestGeneratedRoleControllerAndAdapterCompileGate(t *testing.T) {
	if _, err := NewRoleController(RoleControllerDeps{}); err == nil {
		t.Fatal("Role controller accepted empty dependencies")
	}
	if _, err := NewRoleController(RoleControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{}))}); err == nil {
		t.Fatal("Role controller accepted nil injected Repository")
	}
	if _, err := NewRoleController(RoleControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{})), Repository: NewRoleRepository()}); err == nil {
		t.Fatal("Role controller accepted nil Partial Overrides")
	}
	controller, err := NewRoleController(RoleControllerDeps{
		Store:      testutil.Store(t, repo.New(&gorm.DB{})),
		Repository: NewRoleRepository(),
		Overrides:  roleCompileOverrides{},
	})
	if err != nil {
		t.Fatalf("NewRoleController: %v", err)
	}
	adapter, err := NewRoleServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewRoleServiceAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewRoleServiceAdapter returned nil adapter")
	}
	var _ adminv1connect.RoleServiceHandler = adapter
	var _ func(context.Context, *adminv1.ListRolesRequest) (*adminv1.ListRolesResponse, error) = adapter.ListRoles
	var _ func(context.Context, *adminv1.UpdateRoleMenusRequest) (*adminv1.UpdateRoleMenusResponse, error) = adapter.UpdateRoleMenus
}
