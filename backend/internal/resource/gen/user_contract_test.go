package gen

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestGeneratedUserConverterExcludesPasswordAndRolesAndPreservesPresence(t *testing.T) {
	create, err := UserCreateToModel(&adminv1.CreateUserRequest{
		Username: "user",
		Password: "Secret123",
		Nickname: "昵称",
		RoleIds:  []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("UserCreateToModel: %v", err)
	}
	if create.Password != "" {
		t.Fatalf("generated User converter consumed password: %q", create.Password)
	}

	old := &model.SysUser{ID: 7, Username: "user", Nickname: "old", Email: "old@example.com", Phone: "1", DeptID: new(int64(3)), Remark: "old", Password: "hash"}
	name := "new"
	updated, columns, err := UserUpdateToModel(old, &adminv1.UpdateUserRequest{Nickname: &name, RoleIds: []int64{99}})
	if err != nil {
		t.Fatalf("selective User update: %v", err)
	}
	if updated.Nickname != name || updated.Email != old.Email || updated.DeptID != old.DeptID || updated.Password != old.Password {
		t.Fatalf("User update overwrote omitted/security fields: updated=%+v old=%+v", updated, old)
	}
	if got := columns.Values(); len(got) != 1 || got[0] != UserColumnNickname {
		t.Fatalf("User selective columns = %v, want [%q]", got, UserColumnNickname)
	}
	if entity, err := UserToEntity(&model.SysUser{Password: "hash"}); err != nil {
		t.Fatalf("UserToEntity: %v", err)
	} else if len(entity.RoleIds) != 0 {
		t.Fatalf("generated User entity projected role IDs: %v", entity.RoleIds)
	}
	if err := UserValidateCreate(&adminv1.CreateUserRequest{}); err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("User required username validation = %v", err)
	}
}

func TestGeneratedUserColumnPolicyRejectsSecurityAndJSONDrift(t *testing.T) {
	valid := repo.NewColumnSet(UserColumnNickname, UserColumnEmail, UserColumnPhone, UserColumnDeptId, UserColumnRemark)
	if err := UserValidateUpdateColumns(valid); err != nil {
		t.Fatalf("valid User ColumnSet rejected: %v", err)
	}
	for name, columns := range map[string]repo.ColumnSet{
		"empty":     repo.NewColumnSet(),
		"duplicate": repo.NewColumnSet(UserColumnRemark, UserColumnRemark),
		"username":  repo.NewColumnSet(UserColumnUsername),
		"status":    repo.NewColumnSet(UserColumnStatus),
		"password":  repo.NewColumnSet(repo.Column("password")),
		"role-ids":  repo.NewColumnSet(repo.Column("role_ids")),
		"json-name": repo.NewColumnSet(repo.Column("deptId")),
		"unknown":   repo.NewColumnSet(repo.Column("nickname_json")),
	} {
		if err := UserValidateUpdateColumns(columns); err == nil {
			t.Errorf("invalid User ColumnSet %s unexpectedly passed", name)
		}
	}
	if got := string(UserColumnDeptId); got != "dept_id" {
		t.Fatalf("User DeptID DBName = %q, want dept_id", got)
	}
}

// SQL NULL stays distinct in the model; the scalar API projects it as zero.
func TestUserNullableDepartmentPresence(t *testing.T) {
	old := &model.SysUser{ID: 7, Status: 1}
	entity, err := UserToEntity(old)
	if err != nil || entity.GetDeptId() != 0 {
		t.Fatalf("NULL projection: entity=%v err=%v", entity, err)
	}
	omitted, columns, err := UserUpdateToModel(old, &adminv1.UpdateUserRequest{})
	if err != nil || omitted.DeptID != nil || len(columns.Values()) != 0 {
		t.Fatalf("omitted update: %+v %v %v", omitted, columns, err)
	}
	zero := int64(0)
	updated, columns, err := UserUpdateToModel(old, &adminv1.UpdateUserRequest{DeptId: &zero})
	if err != nil || updated.DeptID == nil || *updated.DeptID != 0 || len(columns.Values()) != 1 {
		t.Fatalf("explicit zero update: %+v %v %v", updated, columns, err)
	}
	_, columns, err = UserUpdateToModel(updated, &adminv1.UpdateUserRequest{DeptId: &zero})
	if err != nil || len(columns.Values()) != 0 {
		t.Fatalf("equal value update: %v %v", columns, err)
	}
}
