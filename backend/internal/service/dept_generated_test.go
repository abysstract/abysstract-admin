package service

import (
	"context"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

func TestDeptHooksPreserveDefaultsAndSpecializedValidationBoundary(t *testing.T) {
	hooks := newDeptHooks()
	created := &model.SysDept{DeptName: "部门"}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, created); err != nil {
		t.Fatalf("default Dept create rejected: %v", err)
	}
	if created.Status != 1 || created.Ancestors != "0" {
		t.Fatalf("Dept create defaults = status=%d ancestors=%q, want 1/0", created.Status, created.Ancestors)
	}

	invalid := &model.SysDept{DeptName: "部门", Email: "not-an-email"}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, invalid); err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid Dept email error = %v code=%v, want InvalidArgument", err, connect.CodeOf(err))
	}

	if err := repo.New(nil).ValidateDeptParent(context.Background(), 7, 7); err != repo.ErrCycle {
		t.Fatalf("self parent error = %v, want ErrCycle", err)
	}

	if _, err := NewGeneratedDeptService(nil); err == nil {
		t.Fatal("NewGeneratedDeptService(nil) unexpectedly succeeded")
	}

	if err := resourcegen.DeptValidateUpdateColumns(repo.NewColumnSet(resourcegen.DeptColumnParentId)); err != nil {
		t.Fatalf("generated Dept parent ColumnPolicy rejected valid column: %v", err)
	}
}

func TestDeptGeneratedUpdateRejectsNoopOnlyAtControllerBoundary(t *testing.T) {
	old := &model.SysDept{ID: 1, DeptName: "部门"}
	updated, columns, err := resourcegen.DeptUpdateToModel(old, &adminv1.UpdateDeptRequest{DeptName: stringPtr("部门")})
	if err != nil {
		t.Fatalf("same-value Dept converter: %v", err)
	}
	if updated.DeptName != old.DeptName || columns.Len() != 0 {
		t.Fatalf("same-value Dept converter = updated=%+v columns=%v, want no columns", updated, columns.Values())
	}
	if err := resourcegen.DeptValidateUpdateColumns(columns); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("empty Dept ColumnSet validation = %v, want empty-patch rejection", err)
	}
}

func stringPtr(value string) *string { return &value }
