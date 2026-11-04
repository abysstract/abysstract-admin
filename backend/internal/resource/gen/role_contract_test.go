package gen

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestGeneratedRoleConverterPreservesPresenceAndNarrowing(t *testing.T) {
	old := &model.SysRole{
		ID:        7,
		RoleName:  "旧角色",
		RoleKey:   "old-role",
		RoleSort:  2,
		DataScope: 1,
		Status:    1,
		Remark:    "旧备注",
	}
	name := "新角色"
	updated, columns, err := RoleUpdateToModel(old, &adminv1.UpdateRoleRequest{RoleName: &name})
	if err != nil {
		t.Fatalf("selective Role update: %v", err)
	}
	if updated.RoleName != name || updated.RoleKey != old.RoleKey || updated.RoleSort != old.RoleSort || updated.DataScope != old.DataScope || updated.Remark != old.Remark {
		t.Fatalf("optional Role update overwrote omitted fields: updated=%+v old=%+v", updated, old)
	}
	if got := columns.Values(); len(got) != 1 || got[0] != RoleColumnRoleName {
		t.Fatalf("Role selective columns = %v, want [%q]", got, RoleColumnRoleName)
	}

	zero := int32(0)
	explicitZero, columns, err := RoleUpdateToModel(old, &adminv1.UpdateRoleRequest{DataScope: &zero})
	if err != nil {
		t.Fatalf("explicit zero DataScope update: %v", err)
	}
	if explicitZero.DataScope != 0 || columns.Len() != 1 || columns.Values()[0] != RoleColumnDataScope {
		t.Fatalf("explicit zero DataScope update = model=%+v columns=%v", explicitZero, columns.Values())
	}

	overflow := int32(32768)
	if _, _, err := RoleUpdateToModel(old, &adminv1.UpdateRoleRequest{DataScope: &overflow}); err == nil || !strings.Contains(err.Error(), "data_scope") {
		t.Fatalf("Role update overflow error = %v, want data_scope error", err)
	}
	if err := RoleValidateCreate(&adminv1.CreateRoleRequest{RoleName: "角色"}); err == nil {
		t.Fatal("Role required role_key unexpectedly passed")
	}
	if _, err := RoleToEntity(&model.SysRole{Status: 99}); err == nil || !strings.Contains(err.Error(), "未知 model 值") {
		t.Fatalf("unknown Role status conversion error = %v, want enum error", err)
	}
}

func TestGeneratedRoleColumnPolicyUsesDBNamesAndRejectsDrift(t *testing.T) {
	valid := repo.NewColumnSet(RoleColumnRoleName, RoleColumnRoleSort, RoleColumnDataScope, RoleColumnRemark)
	if err := RoleValidateUpdateColumns(valid); err != nil {
		t.Fatalf("valid Role ColumnSet rejected: %v", err)
	}
	for name, columns := range map[string]repo.ColumnSet{
		"empty":          repo.NewColumnSet(),
		"duplicate":      repo.NewColumnSet(RoleColumnRemark, RoleColumnRemark),
		"key":            repo.NewColumnSet(RoleColumnId),
		"role-key":       repo.NewColumnSet(RoleColumnRoleKey),
		"status":         repo.NewColumnSet(RoleColumnStatus),
		"server-managed": repo.NewColumnSet(RoleColumnUpdateTime),
		"json-name":      repo.NewColumnSet(repo.Column("roleName")),
		"unknown":        repo.NewColumnSet(repo.Column("role_name_json")),
	} {
		if err := RoleValidateUpdateColumns(columns); err == nil {
			t.Errorf("invalid Role ColumnSet %s unexpectedly passed", name)
		}
	}
	if got := string(RoleColumnRoleName); got != "role_name" {
		t.Fatalf("RoleName DBName = %q, want role_name", got)
	}
}
