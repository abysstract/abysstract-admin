//go:build integration

package gen

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// TestGeneratedRoleRepositoryAdapterDatabaseContract exercises the
// generated scalar adapter itself. Production Role injects a thin typed
// decorator because roles use soft-delete and own authorization relations.
func TestGeneratedRoleRepositoryAdapterDatabaseContract(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	base := repo.New(gdb)
	adapter := NewRoleRepository()
	ctx := context.Background()

	m := &model.SysRole{
		RoleName:  "generated adapter role",
		RoleKey:   "generated-adapter-role",
		RoleSort:  1,
		DataScope: 1,
		Status:    1,
		Remark:    "备注",
	}
	if err := adapter.Create(ctx, base, m); err != nil {
		t.Fatalf("generated Role Create: %v", err)
	}
	if m.ID == 0 || m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
		t.Fatalf("generated Role Create did not sync key/audit fields: %+v", m)
	}

	items, total, err := adapter.List(ctx, base, RoleListQuery{PageNum: 1, PageSize: 10, Keyword: "generated adapter"}, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("generated Role List: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != m.ID {
		t.Fatalf("generated Role List = total=%d items=%v, want one row", total, items)
	}

	before := m.UpdateTime
	m.RoleName = "generated adapter updated"
	if err := adapter.Update(ctx, base, m.ID, m, repo.NewColumnSet(RoleColumnRoleName), repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("generated Role selective Update: %v", err)
	}
	if !m.UpdateTime.After(before) {
		t.Fatalf("generated Role Update did not advance UpdatedAt: before=%s after=%s", before, m.UpdateTime)
	}
	stored, err := adapter.GetByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("generated Role Get: %v", err)
	}
	if stored.RoleName != "generated adapter updated" || stored.RoleKey != m.RoleKey || !stored.UpdateTime.Equal(m.UpdateTime) {
		t.Fatalf("generated Role stored row = %+v, model = %+v", stored, m)
	}

	if err := adapter.DeleteByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("generated Role Delete: %v", err)
	}
	if _, err := adapter.GetByKey(ctx, base, m.ID, repo.UnrestrictedAccessScope()); err != repo.ErrNotFound {
		t.Fatalf("generated Role Get after adapter delete = %v, want NotFound", err)
	}
}
