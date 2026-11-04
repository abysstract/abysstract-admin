//go:build integration

package service

import (
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestProgramKeysAndDictAssociation(t *testing.T) {
	base, done := openUserSecurityIntegration(t)
	defer done()
	ctx := userClaimsContext(1, "admin")
	configs, err := NewGeneratedConfigService(base)
	if err != nil {
		t.Fatal(err)
	}
	dicts, err := NewGeneratedDictService(base)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := NewGeneratedRoleService(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{" KeyA ", "keya", "Key A"} {
		if _, err := configs.CreateConfig(ctx, &adminv1.CreateConfigRequest{ConfigName: key, ConfigKey: key, ConfigValue: "v"}); err != nil {
			t.Fatal(err)
		}
		created, err := roles.CreateRole(ctx, &adminv1.CreateRoleRequest{RoleName: key, RoleKey: key})
		if err != nil {
			t.Fatal(err)
		}
		if key == " KeyA " {
			for i := 0; i < 2; i++ {
				if _, err := roles.DeleteRole(ctx, &adminv1.DeleteRoleRequest{Id: created.Role.Id}); err != nil {
					t.Fatal(err)
				}
				created, err = roles.CreateRole(ctx, &adminv1.CreateRoleRequest{RoleName: "role", RoleKey: " KeyA "})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if _, err := configs.CreateConfig(ctx, &adminv1.CreateConfigRequest{ConfigName: "duplicate", ConfigKey: " KeyA ", ConfigValue: "v"}); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("trim duplicate: %v", err)
	}
	if _, err := roles.CreateRole(ctx, &adminv1.CreateRoleRequest{RoleName: "empty", RoleKey: " \t "}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty key: %v", err)
	}
	for _, key := range []string{"TypeA", "typea"} {
		if _, err := dicts.CreateDictType(ctx, &adminv1.CreateDictTypeRequest{DictName: key, DictType: " " + key + " "}); err != nil {
			t.Fatal(err)
		}
		if _, err := dicts.CreateDictData(ctx, &adminv1.CreateDictDataRequest{DictType: " " + key + " ", DictLabel: key, DictValue: " v "}); err != nil {
			t.Fatal(err)
		}
		filter := " " + key + " "
		list, err := dicts.ListDictData(ctx, &adminv1.ListDictDataRequest{DictType: &filter})
		if err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != 1 || list.Items[0].DictType != key || list.Items[0].DictValue != " v " {
			t.Fatalf("association: %+v", list.Items)
		}
	}
}
