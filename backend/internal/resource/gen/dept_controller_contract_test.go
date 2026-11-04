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

func TestGeneratedDeptControllerAndAdapterCompileGate(t *testing.T) {
	if _, err := NewDeptController(DeptControllerDeps{}); err == nil {
		t.Fatal("Dept controller accepted empty dependencies")
	}
	if _, err := NewDeptController(DeptControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{}))}); err == nil {
		t.Fatal("Dept generated controller accepted nil Repository")
	}
	controller, err := NewDeptController(DeptControllerDeps{
		Store:      testutil.Store(t, repo.New(&gorm.DB{})),
		Repository: NewDeptRepository(),
	})
	if err != nil {
		t.Fatalf("NewDeptController: %v", err)
	}
	adapter, err := NewDeptServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewDeptServiceAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewDeptServiceAdapter returned nil adapter")
	}
	var _ adminv1connect.DeptServiceHandler = adapter
	var _ func(context.Context, *adminv1.ListDeptRowsRequest) (*adminv1.ListDeptRowsResponse, error) = adapter.ListDeptRows
}
