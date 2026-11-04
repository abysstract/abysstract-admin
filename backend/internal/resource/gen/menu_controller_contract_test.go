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

func TestGeneratedMenuControllerAndAdapterCompileGate(t *testing.T) {
	if _, err := NewMenuController(MenuControllerDeps{}); err == nil {
		t.Fatal("Menu controller accepted empty dependencies")
	}
	if _, err := NewMenuController(MenuControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{}))}); err == nil {
		t.Fatal("Menu generated controller accepted nil Repository")
	}
	controller, err := NewMenuController(MenuControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{})), Repository: NewMenuRepository()})
	if err != nil {
		t.Fatalf("NewMenuController: %v", err)
	}
	adapter, err := NewMenuServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewMenuServiceAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewMenuServiceAdapter returned nil adapter")
	}
	var _ adminv1connect.MenuServiceHandler = adapter
	var _ func(context.Context, *adminv1.ListMenusRequest) (*adminv1.ListMenusResponse, error) = adapter.ListMenus
}
