//go:build integration

package gen

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// TestGeneratedDeptRepositoryAdapterDatabaseContract exercises the
// generated standard adapter itself. Production Dept injects a thin typed
// decorator around this adapter because the published sys_dept behavior adds
// del_flag soft deletion and ancestors/child protection in service/repo hooks.
func TestGeneratedDeptRepositoryAdapterDatabaseContract(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	base := repo.New(gdb)
	adapter := NewDeptRepository()
	ctx := context.Background()

	m := &model.SysDept{
		ParentID:  0,
		Ancestors: "0",
		DeptName:  "generated adapter dept",
		OrderNum:  1,
		Leader:    "负责人",
		Phone:     "13800000000",
		Email:     "generated@example.com",
		Status:    1,
	}
	if err := adapter.Create(ctx, base, m); err != nil {
		t.Fatalf("generated Dept Create: %v", err)
	}
	if m.ID == 0 || m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
		t.Fatalf("generated Dept Create did not sync key/audit fields: %+v", m)
	}

	items, total, err := adapter.List(ctx, base, DeptListQuery{PageNum: 1, PageSize: 10, Keyword: "generated adapter"}, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("generated Dept List: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != m.ID {
		t.Fatalf("generated Dept List = total=%d items=%v, want one row", total, items)
	}

	before := m.UpdateTime
	m.DeptName = "generated adapter updated"
	if err := adapter.Update(ctx, base, m.ID, m, repo.NewColumnSet(DeptColumnDeptName), repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("generated Dept selective Update: %v", err)
	}
	if !m.UpdateTime.After(before) {
		t.Fatalf("generated Dept Update did not advance UpdatedAt: before=%s after=%s", before, m.UpdateTime)
	}
	stored, err := adapter.GetByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("generated Dept Get: %v", err)
	}
	if stored.DeptName != "generated adapter updated" || !stored.UpdateTime.Equal(m.UpdateTime) {
		t.Fatalf("generated Dept stored row = %+v, model = %+v", stored, m)
	}

	if err := adapter.DeleteByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("generated Dept Delete: %v", err)
	}
	if _, err := adapter.GetByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope()); err != repo.ErrNotFound {
		t.Fatalf("generated Dept Get after adapter delete = %v, want NotFound", err)
	}
}
