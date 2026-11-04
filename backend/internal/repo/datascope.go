package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

// data_scope 取值（与 sys_role.data_scope 对齐）。
const (
	ScopeAll       int16 = 1 // 全部
	ScopeCustom    int16 = 2 // 自定义（由 sys_role_dept 显式配置部门）
	ScopeDept      int16 = 3 // 本部门
	ScopeDeptBelow int16 = 4 // 本部门及以下
	ScopeSelf      int16 = 5 // 仅本人
)

// VisibleScope 是多角色 data_scope 合并后的可见数据范围（纯数据）。
// 由 ResolveVisibleScope 解析得到，可作为纯函数测试目标，不依赖 DB。
type VisibleScope struct {
	All      bool    // 任一角色为「全部」→ 无限制（短路）
	DeptIDs  []int64 // 合并后的部门集合（本部门 / 及以下 / 自定义），已排序去重
	SelfOnly bool    // 是否包含「仅本人」条件（按 creatorCol = UserID 过滤）
	UserID   int64   // 「仅本人」使用的创建者 ID
}

// RoleScope 描述单个角色对可见范围的贡献（纯数据，便于测试构造）。
type RoleScope struct {
	DataScope int16   // sys_role.data_scope
	DeptIDs   []int64 // 仅 ScopeCustom 使用；空集合贡献空且不报错
}

// ResolveVisibleScope 合并多角色 data_scope 为最终可见范围（纯函数）。
//
// 规则（设计文档 Property 5）：
//   - 任一角色为「全部」(ScopeAll) → VisibleScope.All=true，短路返回（无范围限制）。
//   - 否则取各角色可见范围的并集：
//   - ScopeDept:      贡献 {user.deptID}
//   - ScopeDeptBelow: 贡献 {user.deptID} ∪ descendantsOf(user.deptID)
//   - ScopeCustom:    贡献 role.DeptIDs（空集合贡献空，且不报错）
//   - ScopeSelf:      SelfOnly=true（贡献 creatorCol = UserID 条件）
//   - 无任何有效角色 → SelfOnly=true, DeptIDs 空（Property 7 安全默认：绝不返回全量）。
//
// descendantsOf 由调用方传入（实际场景查 sys_dept.ancestors），属性测试可直接构造内存数据。
func ResolveVisibleScope(userID, deptID int64, roles []RoleScope, descendantsOf func(int64) []int64) VisibleScope {
	vs := VisibleScope{UserID: userID}
	if len(roles) == 0 {
		vs.SelfOnly = true
		return vs
	}

	deptSet := map[int64]struct{}{}
	for _, r := range roles {
		switch r.DataScope {
		case ScopeAll:
			vs.All = true
			return vs
		case ScopeDept:
			deptSet[deptID] = struct{}{}
		case ScopeDeptBelow:
			deptSet[deptID] = struct{}{}
			if descendantsOf != nil {
				for _, d := range descendantsOf(deptID) {
					deptSet[d] = struct{}{}
				}
			}
		case ScopeCustom:
			for _, d := range r.DeptIDs {
				deptSet[d] = struct{}{}
			}
		case ScopeSelf:
			vs.SelfOnly = true
		}
	}
	vs.DeptIDs = sortedKeys(deptSet)
	return vs
}

// IsEmpty 报告该可见范围是否不含任何可见数据（即恒假作用域）。
// All=true 或有 DeptIDs 或 SelfOnly 任一为真 → 不空。
func (vs VisibleScope) IsEmpty() bool {
	return !vs.All && len(vs.DeptIDs) == 0 && !vs.SelfOnly
}

// Scope 把 VisibleScope 翻译为 GORM 作用域（纯函数，依赖 gorm.DB）。
//
// 翻译规则（设计文档 Data Models）：
//   - All=true → 不追加任何条件
//   - 否则 (deptCol IN DeptIDs) OR (SelfOnly AND creatorCol = UserID)
//   - 两者皆空 → WHERE 1=0（恒假，fail-secure，绝不返回全量）
func (vs VisibleScope) Scope(deptCol, creatorCol string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if vs.All {
			return db
		}
		var conds []string
		var args []any
		if len(vs.DeptIDs) > 0 {
			ph := make([]string, len(vs.DeptIDs))
			for i, d := range vs.DeptIDs {
				ph[i] = "?"
				args = append(args, d)
			}
			conds = append(conds, fmt.Sprintf("%s IN (%s)", deptCol, strings.Join(ph, ",")))
		}
		if vs.SelfOnly {
			conds = append(conds, fmt.Sprintf("%s = ?", creatorCol))
			args = append(args, vs.UserID)
		}
		if len(conds) == 0 {
			return db.Where("1 = 0")
		}
		return db.Where(strings.Join(conds, " OR "), args...)
	}
}

// DataScope 解析当前用户的合并 data_scope 并返回 GORM 作用域。
// 由受控列表查询通过 .Scopes() 挂接。
//
// 安全默认（fail-secure）：context 无 Claims 或解析失败时返回恒假作用域，绝不放行全量。
func (r *Repo) DataScope(ctx context.Context, deptCol, creatorCol string) func(*gorm.DB) *gorm.DB {
	claims, ok := interceptor.ClaimsFromContext(ctx)
	if !ok {
		return VisibleScope{}.Scope(deptCol, creatorCol)
	}
	vs, err := r.ResolveScope(ctx, claims.UserID)
	if err != nil {
		return VisibleScope{}.Scope(deptCol, creatorCol)
	}
	return vs.Scope(deptCol, creatorCol)
}

// ResolveScope 从 DB 加载当前用户的角色 data_scope 与（自定义）部门集合，
// 调用纯函数 ResolveVisibleScope 解析为 VisibleScope。
func (r *Repo) ResolveScope(ctx context.Context, userID int64) (VisibleScope, error) {
	var user model.SysUser
	if err := r.db.WithContext(ctx).
		Select("id", "dept_id").
		First(&user, userID).Error; err != nil {
		return VisibleScope{}, err
	}

	q := query.Use(r.db)
	role, ur, rd := q.SysRole, q.SysUserRole, q.SysRoleDept
	var rows []struct {
		RoleID    int64
		DataScope int16
		DeptID    *int64
	}
	err := role.WithContext(ctx).Select(role.ID.As("role_id"), role.DataScope, rd.DeptID).
		Join(ur, ur.RoleID.EqCol(role.ID)).
		LeftJoin(rd, rd.RoleID.EqCol(role.ID)).
		Where(ur.UserID.Eq(userID), role.Status.Eq(1), role.DelFlag.Eq(0)).Scan(&rows)
	if err != nil {
		return VisibleScope{}, err
	}

	var roles []RoleScope
	roleIndexes := make(map[int64]int)
	for _, row := range rows {
		index, ok := roleIndexes[row.RoleID]
		if !ok {
			index = len(roles)
			roleIndexes[row.RoleID] = index
			roles = append(roles, RoleScope{DataScope: row.DataScope})
		}
		if row.DeptID != nil {
			roles[index].DeptIDs = append(roles[index].DeptIDs, *row.DeptID)
		}
	}
	var deptID int64
	if user.DeptID != nil {
		deptID = *user.DeptID
	}
	return ResolveVisibleScope(userID, deptID, roles, r.deptDescendants(ctx)), nil
}

// deptDescendants 返回一个闭包：给定 deptID，依据 sys_dept.ancestors 前缀匹配
// 返回其所有后代部门 ID（不含自身）。属性测试时可用伪造闭包替换。
//
// ancestors 字段格式（0001_init_sys.sql）：顶级 ancestors='0'；
// 子级 ancestors = '<父的 ancestors>,<父 id>'，例如 dept_id=5 父=1 → ancestors='0,1'。
// 因此 deptID 的完整路径 = ancestors + ',' + deptID；后代条件：
//
//	ancestors = '<path>'           （直接子，自身 ancestors 字段等于 path）
//	OR ancestors LIKE '<path>,%'   （更深层的后代）
func (r *Repo) deptDescendants(ctx context.Context) func(int64) []int64 {
	return func(deptID int64) []int64 {
		var dept model.SysDept
		if err := r.db.WithContext(ctx).
			Select("id", "ancestors").
			First(&dept, deptID).Error; err != nil {
			return nil
		}
		path := strings.TrimSpace(dept.Ancestors)
		var prefix string
		if path == "" {
			prefix = fmt.Sprintf("%d", deptID)
		} else {
			prefix = fmt.Sprintf("%s,%d", path, deptID)
		}
		var ids []int64
		r.db.WithContext(ctx).Model(&model.SysDept{}).
			Where("ancestors = ? OR ancestors LIKE ?", prefix, prefix+",%").
			Pluck("id", &ids)
		return ids
	}
}

func sortedKeys(m map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
