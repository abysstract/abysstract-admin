package gen

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestGeneratedDeptConverterPreservesOptionalPresenceAndNarrowEnum(t *testing.T) {
	old := &model.SysDept{
		ID:       7,
		ParentID: 3,
		DeptName: "旧部门",
		OrderNum: 4,
		Leader:   "负责人",
		Phone:    "13800000000",
		Email:    "old@example.com",
		Status:   1,
	}
	name := "新部门"
	updated, columns, err := DeptUpdateToModel(old, &adminv1.UpdateDeptRequest{DeptName: &name})
	if err != nil {
		t.Fatalf("selective Dept update: %v", err)
	}
	if updated.DeptName != name || updated.ParentID != old.ParentID || updated.OrderNum != old.OrderNum || updated.Email != old.Email {
		t.Fatalf("optional Dept update overwrote omitted fields: updated=%+v old=%+v", updated, old)
	}
	if got := columns.Values(); len(got) != 1 || got[0] != DeptColumnDeptName {
		t.Fatalf("Dept selective columns = %v, want [%q]", got, DeptColumnDeptName)
	}

	root := int64(0)
	explicitRoot, columns, err := DeptUpdateToModel(old, &adminv1.UpdateDeptRequest{ParentId: &root})
	if err != nil {
		t.Fatalf("explicit zero ParentID update: %v", err)
	}
	if explicitRoot.ParentID != 0 || columns.Len() != 1 || columns.Values()[0] != DeptColumnParentId {
		t.Fatalf("explicit zero ParentID update = model=%+v columns=%v", explicitRoot, columns.Values())
	}

	if err := DeptValidateCreate(&adminv1.CreateDeptRequest{DeptName: "部门", Email: "invalid"}); err != nil {
		// Schema owns length/required only; format is intentionally Hook-owned.
		t.Fatalf("schema validation rejected a Hook-owned email format: %v", err)
	}
	if err := DeptValidateCreate(&adminv1.CreateDeptRequest{}); err == nil {
		t.Fatal("Dept required dept_name unexpectedly passed")
	}
	if _, err := DeptToEntity(&model.SysDept{Status: 99}); err == nil || !strings.Contains(err.Error(), "未知 model 值") {
		t.Fatalf("unknown Dept status conversion error = %v, want enum error", err)
	}
}

func TestGeneratedDeptColumnPolicyUsesDBNames(t *testing.T) {
	valid := repo.NewColumnSet(
		DeptColumnParentId,
		DeptColumnDeptName,
		DeptColumnOrderNum,
		DeptColumnLeader,
		DeptColumnPhone,
		DeptColumnEmail,
	)
	if err := DeptValidateUpdateColumns(valid); err != nil {
		t.Fatalf("valid Dept ColumnSet rejected: %v", err)
	}
	for name, columns := range map[string]repo.ColumnSet{
		"empty":           repo.NewColumnSet(),
		"duplicate":       repo.NewColumnSet(DeptColumnEmail, DeptColumnEmail),
		"key":             repo.NewColumnSet(repo.Column("id")),
		"ancestors":       repo.NewColumnSet(repo.Column("ancestors")),
		"soft-delete":     repo.NewColumnSet(repo.Column("del_flag")),
		"json-name-drift": repo.NewColumnSet(repo.Column("deptName")),
	} {
		if err := DeptValidateUpdateColumns(columns); err == nil {
			t.Errorf("invalid Dept ColumnSet %s unexpectedly passed", name)
		}
	}
	if got := string(DeptColumnDeptName); got != "dept_name" {
		t.Fatalf("DeptName DBName = %q, want dept_name", got)
	}
	if got := string(DeptColumnParentId); got != "parent_id" {
		t.Fatalf("ParentID DBName = %q, want parent_id", got)
	}
}
