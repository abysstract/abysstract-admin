package service

import (
	"context"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func newMenuIntegrationHandler(t *testing.T, base *repo.Repo) adminv1connect.MenuServiceHandler {
	t.Helper()
	handler, err := NewGeneratedMenuService(base)
	if err != nil {
		t.Fatalf("NewGeneratedMenuService: %v", err)
	}
	return handler
}

func createIntegrationMenu(t *testing.T, handler adminv1connect.MenuServiceHandler, parentID int64, name, menuType string) *adminv1.Menu {
	t.Helper()
	response, err := handler.CreateMenu(context.Background(), &adminv1.CreateMenuRequest{
		MenuName:  name,
		ParentId:  parentID,
		OrderNum:  1,
		Path:      name,
		Component: "system/" + name,
		MenuType:  menuType,
		Perms:     "system:" + name + ":list",
		Icon:      "menu",
	})
	if err != nil {
		t.Fatalf("CreateMenu(%q): %v", name, err)
	}
	if response.Menu == nil || response.Menu.Id == 0 {
		t.Fatalf("CreateMenu(%q) returned no Menu: %v", name, response.Menu)
	}
	return response.Menu
}

func ptrInt64(value int64) *int64 { return &value }
