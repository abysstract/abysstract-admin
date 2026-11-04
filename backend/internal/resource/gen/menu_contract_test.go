package gen

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestGeneratedMenuConverterPreservesPresenceAndNarrowing(t *testing.T) {
	old := &model.SysMenu{
		ID:        7,
		MenuName:  "旧菜单",
		ParentID:  1,
		OrderNum:  2,
		Path:      "old",
		Component: "old/index",
		MenuType:  "C",
		IsFrame:   1,
		IsCache:   1,
		Visible:   1,
		Perms:     "old:read",
		Icon:      "old",
		Status:    1,
	}
	name := "新菜单"
	updated, columns, err := MenuUpdateToModel(old, &adminv1.UpdateMenuRequest{MenuName: &name})
	if err != nil {
		t.Fatalf("selective Menu update: %v", err)
	}
	if updated.MenuName != name || updated.ParentID != old.ParentID || updated.Path != old.Path || updated.MenuType != old.MenuType {
		t.Fatalf("optional Menu update overwrote omitted fields: updated=%+v old=%+v", updated, old)
	}
	if got := columns.Values(); len(got) != 1 || got[0] != MenuColumnMenuName {
		t.Fatalf("Menu selective columns = %v, want [%q]", got, MenuColumnMenuName)
	}

	zero := int32(0)
	explicitZero, columns, err := MenuUpdateToModel(old, &adminv1.UpdateMenuRequest{IsFrame: &zero})
	if err != nil {
		t.Fatalf("explicit zero IsFrame update: %v", err)
	}
	if explicitZero.IsFrame != 0 || columns.Len() != 1 || columns.Values()[0] != MenuColumnIsFrame {
		t.Fatalf("explicit zero IsFrame update = model=%+v columns=%v", explicitZero, columns.Values())
	}

	overflow := int32(32768)
	if _, _, err := MenuUpdateToModel(old, &adminv1.UpdateMenuRequest{IsFrame: &overflow}); err == nil || !strings.Contains(err.Error(), "is_frame") {
		t.Fatalf("Menu update overflow error = %v, want is_frame error", err)
	}
	if err := MenuValidateCreate(&adminv1.CreateMenuRequest{MenuName: "菜单"}); err == nil {
		t.Fatal("Menu required menu_type unexpectedly passed")
	}
	if _, err := MenuToEntity(&model.SysMenu{Status: 99}); err == nil || !strings.Contains(err.Error(), "未知 model 值") {
		t.Fatalf("unknown Menu status conversion error = %v, want enum error", err)
	}
}

func TestGeneratedMenuColumnPolicyUsesDBNames(t *testing.T) {
	valid := repo.NewColumnSet(
		MenuColumnMenuName, MenuColumnParentId, MenuColumnOrderNum,
		MenuColumnPath, MenuColumnComponent, MenuColumnQuery,
		MenuColumnIsFrame, MenuColumnIsCache, MenuColumnMenuType,
		MenuColumnVisible, MenuColumnPerms, MenuColumnIcon,
	)
	if err := MenuValidateUpdateColumns(valid); err != nil {
		t.Fatalf("valid Menu ColumnSet rejected: %v", err)
	}
	for name, columns := range map[string]repo.ColumnSet{
		"empty":     repo.NewColumnSet(),
		"duplicate": repo.NewColumnSet(MenuColumnIcon, MenuColumnIcon),
		"key":       repo.NewColumnSet(MenuColumnId),
		"status":    repo.NewColumnSet(MenuColumnStatus),
		"children":  repo.NewColumnSet(repo.Column("children")),
		"json-name": repo.NewColumnSet(repo.Column("menuName")),
		"unknown":   repo.NewColumnSet(repo.Column("menu_name_json")),
	} {
		if err := MenuValidateUpdateColumns(columns); err == nil {
			t.Errorf("invalid Menu ColumnSet %s unexpectedly passed", name)
		}
	}
	if got := string(MenuColumnMenuName); got != "menu_name" {
		t.Fatalf("MenuName DBName = %q, want menu_name", got)
	}
}
