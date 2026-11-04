package service

import (
	"context"
	"fmt"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func newDeptIntegrationHandler(t *testing.T, base *repo.Repo) adminv1connect.DeptServiceHandler {
	t.Helper()
	handler, err := NewGeneratedDeptService(base)
	if err != nil {
		t.Fatalf("NewGeneratedDeptService: %v", err)
	}
	return handler
}

func createIntegrationDept(t *testing.T, handler adminv1connect.DeptServiceHandler, parentID int64, name string) *adminv1.Dept {
	t.Helper()
	response, err := handler.CreateDept(context.Background(), &adminv1.CreateDeptRequest{
		ParentId: parentID,
		DeptName: name,
		OrderNum: 1,
		Leader:   "负责人",
		Phone:    "13800000000",
		Email:    name + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateDept(%q): %v", name, err)
	}
	if response.Dept == nil || response.Dept.Id == 0 {
		t.Fatalf("CreateDept(%q) returned no Dept: %v", name, response.Dept)
	}
	return response.Dept
}

func stringInt64(value int64) string {
	return fmt.Sprintf("%d", value)
}
