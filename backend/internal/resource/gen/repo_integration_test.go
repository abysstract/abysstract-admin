//go:build integration

package gen

import (
	"context"
	"errors"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// setupRepo 迁移并返回生成 adapter + repo；DB 不可达时 skip。
func setupRepo(t *testing.T) (*DictTypeRepository, *repo.Repo) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewDictTypeRepository(), repo.New(gdb)
}

// 10.2b：deny-all AccessScope 下，非空 QueryClause 不得读出任何记录。
func TestGeneratedListDenyAllIgnoresNonEmptyClauses(t *testing.T) {
	a, r := setupRepo(t)
	seedDictType(t, a, r, "十甲", "zz_gen_test_a")

	var denyAll repo.AccessScope
	items, total, err := a.List(context.Background(), r, DictTypeListQuery{
		PageNum: 1, PageSize: 10,
		Keyword: "十甲", // 非空 Search QueryClause
		Orders:  []repo.Order{{Column: DictTypeColumnId, Desc: true}},
	}, denyAll)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("deny-all must return nothing even with non-empty clauses, got total=%d items=%d", total, len(items))
	}
}

// 10.2b：unrestricted 下 Search/Order 语义生效且参数化。
func TestGeneratedListUnrestrictedAppliesSearchAndOrder(t *testing.T) {
	a, r := setupRepo(t)
	seedDictType(t, a, r, "十甲", "zz_gen_test_a")
	seedDictType(t, a, r, "十乙", "zz_gen_test_b")

	items, total, err := a.List(context.Background(), r, DictTypeListQuery{
		PageNum: 1, PageSize: 10, Keyword: " 十乙 ",
	}, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].DictName != "十乙" {
		t.Fatalf("keyword search wrong: total=%d items=%v", total, items)
	}
}

// 评审点：Create 后必须把数据库默认值/审计字段同步回同一个模型，供 ToEntity/AfterCommit 复用。
func TestGeneratedCreateSyncsDatabaseDefaultsBackToModel(t *testing.T) {
	a, r := setupRepo(t)
	m := &model.SysDictType{DictName: "创建回填", DictType: "t_create_sync"}
	if err := a.Create(context.Background(), r, m); err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.ID == 0 {
		t.Fatal("create must populate primary key")
	}
	if m.Status != 1 {
		t.Fatalf("model Status = %d, want DB default 1", m.Status)
	}
	if m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
		t.Fatalf("model audit fields must be populated, got create=%v update=%v", m.CreateTime, m.UpdateTime)
	}
	entity, err := DictTypeToEntity(m)
	if err != nil {
		t.Fatalf("DictTypeToEntity(create model) error = %v", err)
	}
	if entity.GetStatus().Number() != 1 {
		t.Fatalf("entity status = %v, want enabled", entity.GetStatus())
	}
	stored, err := a.GetByKey(context.Background(), r, m.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Status != m.Status || !stored.CreateTime.Equal(m.CreateTime) || !stored.UpdateTime.Equal(m.UpdateTime) {
		t.Fatalf("stored row = %+v, model after create = %+v; want synced snapshot", *stored, *m)
	}
}

// 10.2c：Update 在同事务内 scoped GetByKeyForUpdate 后写入，并刷新 UpdatedAt。
func TestGeneratedUpdateLocksAndRefreshesUpdatedAt(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "原", "t_update")
	oldUpdatedAt := m.UpdateTime

	m.DictName = "新"
	if err := a.Update(context.Background(), r, m.ID, m, repo.NewColumnSet(DictTypeColumnDictName), repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !m.UpdateTime.After(oldUpdatedAt) {
		t.Fatalf("model UpdatedAt must be refreshed, before=%v after=%v", oldUpdatedAt, m.UpdateTime)
	}
	stored, err := a.GetByKey(context.Background(), r, m.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.DictName != "新" {
		t.Fatalf("update not persisted: %q", stored.DictName)
	}
	if !stored.UpdateTime.Equal(m.UpdateTime) {
		t.Fatalf("persisted UpdatedAt = %v, model UpdatedAt = %v; want the single adapter now value", stored.UpdateTime, m.UpdateTime)
	}
}

// 10.2c：deny-all scope 下 Update 表现为 NotFound 且不写任何记录。
func TestGeneratedUpdateDenyAllIsNotFoundAndNoop(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "原", "t_deny")

	var denyAll repo.AccessScope
	m.DictName = "改"
	err := a.Update(context.Background(), r, m.ID, m, repo.NewColumnSet(DictTypeColumnDictName), denyAll)
	if err == nil {
		t.Fatal("deny-all update must fail")
	}
	stored, gerr := a.GetByKey(context.Background(), r, m.ID, repo.UnrestrictedAccessScope())
	if gerr != nil {
		t.Fatalf("get: %v", gerr)
	}
	if stored.DictName != "原" {
		t.Fatalf("deny-all update must not write, got %q", stored.DictName)
	}
}

// 10.2c：scope 不可见时 Delete 同样 NotFound 且不删记录。
func TestGeneratedDeleteDenyAllIsNotFoundAndNoop(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "原", "t_del")

	var denyAll repo.AccessScope
	if err := a.DeleteByKey(context.Background(), r, m.ID, denyAll); err == nil {
		t.Fatal("deny-all delete must fail")
	}
	if _, err := a.GetByKey(context.Background(), r, m.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("record must survive deny-all delete: %v", err)
	}
}

// 评审点 1：BatchDelete 含不存在 ID 时必须整体 NotFound，可删的记录也不得删除。
func TestGeneratedBatchDeletePartialMissingIsAllOrNothing(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "批一", "t_batch_partial")

	ctx := context.Background()
	err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		_, err := a.BatchDelete(ctx, txRepo, []int64{m.ID, 999999999}, repo.UnrestrictedAccessScope())
		return err // 返回 error → rollback
	})
	if err == nil {
		t.Fatal("batch delete with a missing ID must fail")
	}
	if _, gerr := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope()); gerr != nil {
		t.Fatalf("existing record must survive all-or-nothing batch delete: %v", gerr)
	}
}

// 评审点 1：deny-all 下 BatchDelete 不得部分删除任何记录。
func TestGeneratedBatchDeleteDenyAllDeletesNothing(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "批二", "t_batch_deny")

	var denyAll repo.AccessScope
	ctx := context.Background()
	err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		_, err := a.BatchDelete(ctx, txRepo, []int64{m.ID}, denyAll)
		return err
	})
	if err == nil {
		t.Fatal("deny-all batch delete must fail")
	}
	if _, gerr := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope()); gerr != nil {
		t.Fatalf("record must survive deny-all batch delete: %v", gerr)
	}
}

// 评审点 1：空集合与重复 ID 的契约。
func TestGeneratedBatchDeleteEmptyAndDuplicateIDs(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "批三", "t_batch_dup")

	ctx := context.Background()
	if _, err := a.BatchDelete(ctx, r, nil, repo.UnrestrictedAccessScope()); err == nil {
		t.Fatal("empty ID set must be rejected")
	}
	// 重复 ID 去重后按一条删除并提交成功。
	var affected int64
	err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		n, err := a.BatchDelete(ctx, txRepo, []int64{m.ID, m.ID}, repo.UnrestrictedAccessScope())
		affected = n
		return err
	})
	if err != nil {
		t.Fatalf("duplicate IDs must dedupe and succeed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}
	if _, gerr := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope()); gerr == nil {
		t.Fatal("record must be deleted after committed batch delete")
	}
}

// 评审点 2：adapter 不自开事务——调用方 rollback 后写入必须不可见。
func TestGeneratedUpdateRunsOnCallerRepoHandle(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "原值", "t_tx_boundary")

	ctx := context.Background()
	err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		m.DictName = "回滚值"
		if err := a.Update(ctx, txRepo, m.ID, m, repo.NewColumnSet(DictTypeColumnDictName), repo.UnrestrictedAccessScope()); err != nil {
			t.Fatalf("update: %v", err)
		}
		return errRollbackForTest
	})
	if err != errRollbackForTest {
		t.Fatalf("tx must return caller error, got %v", err)
	}
	stored, gerr := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope())
	if gerr != nil {
		t.Fatalf("get: %v", gerr)
	}
	if stored.DictName != "原值" {
		t.Fatalf("caller rollback must discard adapter writes, got %q", stored.DictName)
	}
}

// 10.4: every protected generated-adapter operation must pass its AccessScope
// through to the generic repository boundary. A zero scope is deny-all even
// when the target record exists.
func TestGeneratedProtectedOperationsAlwaysApplyAccessScope(t *testing.T) {
	a, r := setupRepo(t)
	m := seedDictType(t, a, r, "受保护", "t_protected_scope")
	ctx := context.Background()
	var denyAll repo.AccessScope

	if _, err := a.GetByKey(ctx, r, m.ID, denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("GetByKey deny-all error = %v, want ErrNotFound", err)
	}
	if err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		_, err := a.GetByKeyForUpdate(ctx, txRepo, m.ID, denyAll)
		return err
	}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("GetByKeyForUpdate deny-all error = %v, want ErrNotFound", err)
	}

	items, total, err := a.List(ctx, r, DictTypeListQuery{PageNum: 1, PageSize: 10, Keyword: "受保护"}, denyAll)
	if err != nil {
		t.Fatalf("List deny-all error = %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("List deny-all = total %d, items %d; want empty", total, len(items))
	}

	m.DictName = "不可写"
	if err := a.Update(ctx, r, m.ID, m, repo.NewColumnSet(DictTypeColumnDictName), denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Update deny-all error = %v, want ErrNotFound", err)
	}
	if err := a.DeleteByKey(ctx, r, m.ID, denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("DeleteByKey deny-all error = %v, want ErrNotFound", err)
	}
	if err := r.InTransaction(ctx, func(txRepo *repo.Repo) error {
		_, err := a.BatchDelete(ctx, txRepo, []int64{m.ID}, denyAll)
		return err
	}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("BatchDelete deny-all error = %v, want ErrNotFound", err)
	}

	stored, err := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("unrestricted GetByKey after deny-all operations: %v", err)
	}
	if stored.DictName != "受保护" {
		t.Fatalf("deny-all Update changed row to %q", stored.DictName)
	}
}

// 10.10: adapter policy rejection happens before its audit timestamp assignment
// or generated Update call, so all invalid columns leave persistent state intact.
func TestGeneratedUpdateColumnPolicyRejectsInvalidColumnsWithoutSQLWrite(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		columns repo.ColumnSet
	}{
		{name: "empty", columns: repo.NewColumnSet()},
		{name: "duplicate", columns: repo.NewColumnSet(DictTypeColumnDictName, DictTypeColumnDictName)},
		{name: "unknown", columns: repo.NewColumnSet(repo.Column("unknown_column"))},
		{name: "key", columns: repo.NewColumnSet(DictTypeColumnId)},
		{name: "server managed", columns: repo.NewColumnSet(DictTypeColumnUpdateTime)},
		{name: "create only", columns: repo.NewColumnSet(repo.Column("dict_type"))},
		{name: "read only", columns: repo.NewColumnSet(repo.Column("status"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := seedDictType(t, a, r, "原始", "t_policy_"+tc.name)
			before, err := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope())
			if err != nil {
				t.Fatalf("get before: %v", err)
			}

			m.DictName = "不应写入"
			if err := a.Update(ctx, r, m.ID, m, tc.columns, repo.UnrestrictedAccessScope()); err == nil {
				t.Fatal("invalid ColumnSet must be rejected")
			}
			after, err := a.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope())
			if err != nil {
				t.Fatalf("get after: %v", err)
			}
			if after.DictName != before.DictName || !after.UpdateTime.Equal(before.UpdateTime) {
				t.Fatalf("invalid columns wrote data: before=%+v after=%+v", before, after)
			}
		})
	}
}

// 10.6: the same deny-all contract applies when a controller receives an
// injected typed decorator rather than the generated adapter directly.
func TestInjectedRepositoryDenyAllSecuritySuite(t *testing.T) {
	base, r := setupRepo(t)
	var injected resource.Repository[model.SysDictType, DictTypeListQuery] = &injectedDictTypeRepository{Repository: base}
	ctx := context.Background()
	m := &model.SysDictType{DictName: "注入安全", DictType: "t_injected_deny", Status: 1}
	if err := injected.Create(ctx, r, m); err != nil {
		t.Fatalf("seed through injected repository: %v", err)
	}
	var denyAll repo.AccessScope

	if _, err := injected.GetByKey(ctx, r, m.ID, denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("GetByKey deny-all = %v, want ErrNotFound", err)
	}
	if err := r.InTransaction(ctx, func(tx *repo.Repo) error {
		_, err := injected.GetByKeyForUpdate(ctx, tx, m.ID, denyAll)
		return err
	}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("GetByKeyForUpdate deny-all = %v, want ErrNotFound", err)
	}
	items, total, err := injected.List(ctx, r, DictTypeListQuery{PageNum: 1, PageSize: 10, Keyword: "注入安全"}, denyAll)
	if err != nil || len(items) != 0 || total != 0 {
		t.Fatalf("List deny-all = items=%d total=%d err=%v", len(items), total, err)
	}
	m.DictName = "不应写入"
	if err := injected.Update(ctx, r, m.ID, m, repo.NewColumnSet(DictTypeColumnDictName), denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Update deny-all = %v, want ErrNotFound", err)
	}
	if err := injected.DeleteByKey(ctx, r, m.ID, denyAll); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("DeleteByKey deny-all = %v, want ErrNotFound", err)
	}
	if err := r.InTransaction(ctx, func(tx *repo.Repo) error {
		_, err := injected.BatchDelete(ctx, tx, []int64{m.ID}, denyAll)
		return err
	}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("BatchDelete deny-all = %v, want ErrNotFound", err)
	}
	stored, err := base.GetByKey(ctx, r, m.ID, repo.UnrestrictedAccessScope())
	if err != nil || stored.DictName != "注入安全" {
		t.Fatalf("deny-all decorator changed row: stored=%+v err=%v", stored, err)
	}
}
