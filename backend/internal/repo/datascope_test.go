package repo

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"
)

// row 模拟 sys_user 的 (dept_id, create_by)，用于纯函数过滤测试。
type row struct {
	deptID   int64
	createBy int64
}

// filterByScope 模拟 SQL 的 (dept_id IN DeptIDs) OR (SelfOnly AND create_by=UserID)。
// 与 VisibleScope.Scope 翻译的 SQL 语义等价；在测试中独立实现，作为对照参考。
func filterByScope(vs VisibleScope, rows []row) []row {
	if vs.All {
		return rows
	}
	deptSet := map[int64]struct{}{}
	for _, d := range vs.DeptIDs {
		deptSet[d] = struct{}{}
	}
	var out []row
	for _, r := range rows {
		if _, ok := deptSet[r.deptID]; ok {
			out = append(out, r)
			continue
		}
		if vs.SelfOnly && r.createBy == vs.UserID {
			out = append(out, r)
		}
	}
	return out
}

// Feature: open-source-readiness, Property 5: data_scope 多角色合并范围
//
// 对任意当前用户、角色集合（各带 data_scope 与自定义部门）、部门树与一批数据行，
// ResolveVisibleScope 合并出的 VisibleScope 应用到数据行后，可见集合应等于规格定义：
//   - 任一角色 ScopeAll → 无范围限制（全集）
//   - 否则各角色可见范围的并集：本部门 / 本部门及以下 / 自定义 / 仅本人
//   - 合并后部门集合为空且无「仅本人」→ 空结果（绝不返回全集）
func TestDataScopeMerge_Property5(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	// 生成稳定的部门树：ID 1..10，ancestors 由 parent 链推导。
	const numDepts = 10
	depts := buildDepts(numDepts, rng)

	// 「本部门及以下」后代闭包：基于已构建的 ancestors 字段做前缀匹配（与生产代码同语义）。
	descendantsOf := func(deptID int64) []int64 {
		var dept deptRow
		for _, d := range depts {
			if d.id == deptID {
				dept = d
				break
			}
		}
		if dept.id == 0 {
			return nil
		}
		prefix := prefixOf(dept)
		var out []int64
		for _, d := range depts {
			if d.id == deptID {
				continue
			}
			if d.ancestors == prefix || hasPrefix(d.ancestors, prefix+",") {
				out = append(out, d.id)
			}
		}
		return out
	}

	// 数据池：50 行随机 dept_id / create_by。
	const numRows = 50
	rows := make([]row, numRows)
	for i := range rows {
		rows[i] = row{
			deptID:   int64(rng.Intn(numDepts) + 1),
			createBy: int64(rng.Intn(5) + 1), // user IDs 1..5
		}
	}

	// genRoles 生成 0..4 个角色，data_scope 在 1..5 间，自定义部门取自 1..numDepts。
	genRoles := func(rng *rand.Rand) []RoleScope {
		n := rng.Intn(5)
		roles := make([]RoleScope, n)
		for i := range roles {
			roles[i] = RoleScope{
				DataScope: int16(rng.Intn(5) + 1),
			}
			if roles[i].DataScope == ScopeCustom {
				k := rng.Intn(4)
				ids := make([]int64, 0, k)
				for j := 0; j < k; j++ {
					ids = append(ids, int64(rng.Intn(numDepts)+1))
				}
				roles[i].DeptIDs = ids
			}
		}
		return roles
	}

	// 属性函数：比较 ResolveVisibleScope + filterByScope 与独立参考实现 expectedMerge 的结果。
	prop := func(userSeed, deptSeed, roleSeed int32) bool {
		rng := rand.New(rand.NewSource(int64(userSeed) ^ int64(deptSeed)<<7 ^ int64(roleSeed)<<14))
		userID := int64(rng.Intn(5) + 1)
		deptID := int64(rng.Intn(numDepts) + 1)
		roles := genRoles(rng)

		vs := ResolveVisibleScope(userID, deptID, roles, descendantsOf)
		got := filterByScope(vs, rows)
		want := expectedMerge(userID, deptID, roles, descendantsOf, rows)

		// 全集或空集：用长度比较 + 同集合判定。
		if len(got) != len(want) {
			return false
		}
		// 以 deptID,createBy 排序后逐项比较。
		sortRows(got)
		sortRows(want)
		return reflect.DeepEqual(got, want)
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 5 failed: %v", err)
	}

	// 显式覆盖关键边界用例。
	t.Run("any_all_short_circuits", func(t *testing.T) {
		vs := ResolveVisibleScope(1, 1, []RoleScope{
			{DataScope: ScopeDept},
			{DataScope: ScopeAll},
			{DataScope: ScopeSelf},
		}, nil)
		if !vs.All || vs.SelfOnly || len(vs.DeptIDs) != 0 {
			t.Fatalf("expected All short-circuit, got %+v", vs)
		}
	})
	t.Run("custom_empty_contributes_nothing", func(t *testing.T) {
		vs := ResolveVisibleScope(1, 1, []RoleScope{
			{DataScope: ScopeCustom, DeptIDs: nil},
			{DataScope: ScopeDept},
		}, nil)
		// 自定义空集贡献空，但本部门应贡献 deptID=1。
		if len(vs.DeptIDs) != 1 || vs.DeptIDs[0] != 1 {
			t.Fatalf("expected [1], got %+v", vs)
		}
	})
	t.Run("all_empty_returns_empty", func(t *testing.T) {
		vs := ResolveVisibleScope(1, 1, []RoleScope{
			{DataScope: ScopeCustom, DeptIDs: nil},
		}, nil)
		if !vs.IsEmpty() {
			t.Fatalf("expected empty scope (WHERE 1=0), got %+v", vs)
		}
	})
}

// Feature: open-source-readiness, Property 6: data_scope 部门树后代遍历
//
// 对任意部门树与任意所属部门，「本部门及以下」(ScopeDeptBelow) 解析出的可见部门集合
// 应恰好等于该部门及其在树中任意层级的全部后代（依据 sys_dept.ancestors 祖先链），
// 不包含非后代部门。
func TestDataScopeDescendants_Property6(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	const numDepts = 12
	depts := buildDepts(numDepts, rng)

	descendantsOf := func(deptID int64) []int64 {
		var dept deptRow
		for _, d := range depts {
			if d.id == deptID {
				dept = d
				break
			}
		}
		if dept.id == 0 {
			return nil
		}
		prefix := prefixOf(dept)
		var out []int64
		for _, d := range depts {
			if d.id == deptID {
				continue
			}
			if d.ancestors == prefix || hasPrefix(d.ancestors, prefix+",") {
				out = append(out, d.id)
			}
		}
		return out
	}

	// 独立参考实现：用 parent 链 BFS 找所有后代，不依赖 ancestors 字符串前缀匹配。
	descendantsByParent := func(deptID int64) []int64 {
		children := map[int64][]int64{}
		for _, d := range depts {
			children[d.parent] = append(children[d.parent], d.id)
		}
		var out []int64
		queue := []int64{deptID}
		seen := map[int64]struct{}{deptID: {}}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, c := range children[cur] {
				if _, ok := seen[c]; ok {
					continue
				}
				seen[c] = struct{}{}
				out = append(out, c)
				queue = append(queue, c)
			}
		}
		return out
	}

	prop := func(deptID32 int32) bool {
		// int32 可能为负：先映射到 1..numDepts。
		deptID := int64(((int(deptID32)%numDepts)+numDepts)%numDepts) + 1
		// ResolveVisibleScope 用 ScopeDeptBelow 单角色：DeptIDs = {deptID} ∪ descendants。
		vs := ResolveVisibleScope(1, deptID, []RoleScope{{DataScope: ScopeDeptBelow}}, descendantsOf)
		if vs.All || vs.SelfOnly {
			return false
		}
		want := append([]int64{deptID}, descendantsByParent(deptID)...)
		return intSliceEqualSorted(vs.DeptIDs, want)
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 6 failed: %v", err)
	}
}

// Feature: open-source-readiness, Property 7: data_scope 安全默认
//
// 对没有任何具备有效 data_scope 角色的用户，受控列表查询的可见范围应退化为「仅本人」
// （creatorCol = 当前用户），绝不返回不受限的全量数据。
func TestDataScopeSafeDefault_Property7(t *testing.T) {
	prop := func(userID, deptID int64) bool {
		// 空角色列表：应退化为仅本人。
		vs := ResolveVisibleScope(userID, deptID, nil, nil)
		if vs.All || len(vs.DeptIDs) != 0 || !vs.SelfOnly || vs.UserID != userID {
			return false
		}
		// 全 ScopeCustom 且 DeptIDs 空：合并后部门集合为空，无 SelfOnly → 恒假（空结果）。
		vs2 := ResolveVisibleScope(userID, deptID, []RoleScope{
			{DataScope: ScopeCustom, DeptIDs: nil},
		}, nil)
		if vs2.All || vs2.SelfOnly || len(vs2.DeptIDs) != 0 {
			return false
		}
		// IsEmpty() 应判定为 true，Scope 翻译为 WHERE 1=0。
		return vs2.IsEmpty()
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 7 failed: %v", err)
	}

	// 显式：仅本人过滤应用到一批数据，只应返回 create_by=userID 的行。
	t.Run("self_only_filters_correctly", func(t *testing.T) {
		vs := ResolveVisibleScope(7, 1, nil, nil)
		rows := []row{{1, 1}, {1, 7}, {2, 7}, {3, 8}}
		got := filterByScope(vs, rows)
		want := []row{{1, 7}, {2, 7}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
}

// ---------- 辅助函数 ----------

type deptRow struct {
	id        int64
	parent    int64
	ancestors string
}

// buildDepts 构造一棵随机但合法的部门树（id=1..n），ancestors 字段与 0001_init_sys.sql 约定一致。
func buildDepts(n int, rng *rand.Rand) []deptRow {
	depts := make([]deptRow, 0, n)
	for i := 1; i <= n; i++ {
		var parent int64
		if i == 1 {
			parent = 0
		} else {
			parent = int64(rng.Intn(i-1) + 1) // 已存在的部门
		}
		var ancestors string
		if parent == 0 {
			ancestors = "0"
		} else {
			for _, d := range depts {
				if d.id == parent {
					ancestors = prefixOf(d)
					break
				}
			}
		}
		depts = append(depts, deptRow{id: int64(i), parent: parent, ancestors: ancestors})
	}
	return depts
}

// prefixOf 返回 dept 在 ancestors 树中的完整路径（即其后代的 ancestors 前缀）。
func prefixOf(d deptRow) string {
	if d.ancestors == "" {
		return fmt.Sprintf("%d", d.id)
	}
	return fmt.Sprintf("%s,%d", d.ancestors, d.id)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// expectedMerge 是 ResolveVisibleScope 的独立参考实现：用显式的 set 操作计算期望集合。
// 与生产代码不同风格，避免循环证明。
func expectedMerge(userID, deptID int64, roles []RoleScope, descendantsOf func(int64) []int64, rows []row) []row {
	if len(roles) == 0 {
		// 安全默认：仅本人
		return filterByScope(VisibleScope{SelfOnly: true, UserID: userID}, rows)
	}
	hasAll := false
	selfOnly := false
	deptSet := map[int64]struct{}{}
	for _, r := range roles {
		switch r.DataScope {
		case ScopeAll:
			hasAll = true
		case ScopeSelf:
			selfOnly = true
		case ScopeDept:
			deptSet[deptID] = struct{}{}
		case ScopeDeptBelow:
			deptSet[deptID] = struct{}{}
			for _, d := range descendantsOf(deptID) {
				deptSet[d] = struct{}{}
			}
		case ScopeCustom:
			for _, d := range r.DeptIDs {
				deptSet[d] = struct{}{}
			}
		}
	}
	if hasAll {
		return rows
	}
	var out []row
	for _, r := range rows {
		if selfOnly && r.createBy == userID {
			out = append(out, r)
			continue
		}
		if _, ok := deptSet[r.deptID]; ok {
			out = append(out, r)
		}
	}
	return out
}

func sortRows(rows []row) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].deptID != rows[j].deptID {
			return rows[i].deptID < rows[j].deptID
		}
		return rows[i].createBy < rows[j].createBy
	})
}

func intSliceEqualSorted(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]int64(nil), a...)
	bc := append([]int64(nil), b...)
	sort.Slice(ac, func(i, j int) bool { return ac[i] < ac[j] })
	sort.Slice(bc, func(i, j int) bool { return bc[i] < bc[j] })
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
