package service

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

func TestMenuHooksPreserveDefaultsAndMenuTypeBoundary(t *testing.T) {
	hooks := newMenuHooks()
	created := &model.SysMenu{MenuName: "菜单", MenuType: "C"}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, created); err != nil {
		t.Fatalf("default Menu create rejected: %v", err)
	}
	if created.Status != 1 || created.Visible != 1 {
		t.Fatalf("Menu create defaults = status=%d visible=%d, want 1/1", created.Status, created.Visible)
	}
	invalid := &model.SysMenu{MenuName: "菜单", MenuType: "X"}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, invalid); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid Menu type code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	if err := repo.ValidateMenuType("F"); err != nil {
		t.Fatalf("valid button MenuType rejected: %v", err)
	}
	if err := repo.ValidateMenuType("X"); err == nil {
		t.Fatal("invalid MenuType unexpectedly accepted")
	}
	if _, err := NewGeneratedMenuService(nil); err == nil {
		t.Fatal("NewGeneratedMenuService(nil) unexpectedly succeeded")
	}
	if err := resourcegen.MenuValidateUpdateColumns(repo.NewColumnSet(resourcegen.MenuColumnParentId)); err != nil {
		t.Fatalf("generated Menu parent ColumnPolicy rejected valid column: %v", err)
	}
}
