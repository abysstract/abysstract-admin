package interceptor

import (
	"fmt"
	"sort"
	"sync"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

// AuthSentinel 表示仅需登录的权限哨兵，HasPerm 对 "@authenticated" 视为已登录即放行。
const AuthSentinel = permission.Authenticated

// PermissionEntry 是公共 permission.Entry 的类型别名，保持现有登记方法签名不变。
type PermissionEntry = permission.Entry

var _ permission.Registrar = (*PermRegistry)(nil)

// PermRegistry 是应用启动期间构造、冻结后只读的 procedure 权限注册表。
// 它是 Auth 与 Permission interceptor 共享的唯一运行时权限 SSOT。
type PermRegistry struct {
	mu       sync.RWMutex
	public   map[string]struct{}
	required map[string]string
	frozen   bool
}

// PermRegistrySnapshot 是 Registry 的只读副本，供完整性测试和诊断枚举。
// 所有集合均为副本，调用方修改不会影响 Registry。
type PermRegistrySnapshot struct {
	PublicProcedures    []string
	RequiredPermissions map[string]string
	Frozen              bool
}

// NewPermRegistry 创建一个可在 bootstrap 阶段登记 procedure 的 Registry。
func NewPermRegistry() *PermRegistry {
	return &PermRegistry{
		public:   make(map[string]struct{}),
		required: make(map[string]string),
	}
}

// RegisterPublic 登记无需登录的 procedure。相同 procedure 可幂等重复登记，
// 已有 required permission 的 procedure 不得改为公开；Freeze 后拒绝一切修改。
func (r *PermRegistry) RegisterPublic(procedures ...string) error {
	return r.register(procedures, nil)
}

// RegisterGenerated 原子登记生成 Resource 导出的权限映射。
// 相同 procedure+permission 幂等；冲突映射、覆盖公开 procedure、空值和冻结后写入均失败。
func (r *PermRegistry) RegisterGenerated(entries ...PermissionEntry) error {
	return r.register(nil, entries)
}

// register 在复制后的 map 上完成全部校验，确认成功后才一次替换内部状态。
// 因此任一 batch 出错不会泄漏部分登记。
func (r *PermRegistry) register(publicProcedures []string, entries []PermissionEntry) error {
	if r == nil {
		return fmt.Errorf("permission registry is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("permission registry is frozen")
	}

	public := clonePublic(r.public)
	required := cloneRequired(r.required)
	for _, procedure := range publicProcedures {
		if procedure == "" {
			return fmt.Errorf("public procedure is empty")
		}
		if _, exists := required[procedure]; exists {
			return fmt.Errorf("public procedure %q conflicts with required permission", procedure)
		}
		public[procedure] = struct{}{}
	}
	for _, entry := range entries {
		if entry.Procedure == "" {
			return fmt.Errorf("permission entry procedure is empty")
		}
		if entry.Permission == "" {
			return fmt.Errorf("permission entry %q has empty permission", entry.Procedure)
		}
		if _, exists := public[entry.Procedure]; exists {
			return fmt.Errorf("permission entry %q cannot override public procedure", entry.Procedure)
		}
		if current, exists := required[entry.Procedure]; exists && current != entry.Permission {
			return fmt.Errorf("permission entry %q conflicts: %q != %q", entry.Procedure, current, entry.Permission)
		}
		required[entry.Procedure] = entry.Permission
	}

	r.public = public
	r.required = required
	return nil
}

// Freeze 关闭所有后续登记。重复 Freeze 是安全的幂等操作。
func (r *PermRegistry) Freeze() error {
	if r == nil {
		return fmt.Errorf("permission registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	return nil
}

// Lookup 返回 procedure 的授权分类，并标识 Registry 是否已冻结可供 interceptor 使用。
func (r *PermRegistry) Lookup(procedure string) (public bool, permission string, frozen bool) {
	if r == nil {
		return false, "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, public = r.public[procedure]
	permission = r.required[procedure]
	return public, permission, r.frozen
}

// IsPublic 返回 procedure 是否公开。interceptor 应使用 Lookup，以同时验证冻结状态。
func (r *PermRegistry) IsPublic(procedure string) bool {
	public, _, _ := r.Lookup(procedure)
	return public
}

// RequiredPermission 返回 procedure 的权限标识和是否已声明。
func (r *PermRegistry) RequiredPermission(procedure string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	permission, ok := r.required[procedure]
	return permission, ok
}

// Snapshot 返回 Registry 当前状态的深拷贝。
func (r *PermRegistry) Snapshot() PermRegistrySnapshot {
	if r == nil {
		return PermRegistrySnapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	public := make([]string, 0, len(r.public))
	for procedure := range r.public {
		public = append(public, procedure)
	}
	sort.Strings(public)
	return PermRegistrySnapshot{
		PublicProcedures:    public,
		RequiredPermissions: cloneRequired(r.required),
		Frozen:              r.frozen,
	}
}

func clonePublic(source map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(source))
	for procedure := range source {
		out[procedure] = struct{}{}
	}
	return out
}

func cloneRequired(source map[string]string) map[string]string {
	out := make(map[string]string, len(source))
	for procedure, permission := range source {
		out[procedure] = permission
	}
	return out
}

// RegisterBuiltinPermissions migrates all pre-Resource RPC permissions into a
// caller-owned Registrar. The system module and remaining bootstrap tests
// register generated Resource permissions on the same registrar before Freeze.
func RegisterBuiltinPermissions(reg permission.Registrar) error {
	if reg == nil {
		return fmt.Errorf("permission registry is nil")
	}
	if err := reg.RegisterPublic(builtinPublicProcedures...); err != nil {
		return err
	}
	return reg.RegisterGenerated(builtinPermissionEntries...)
}

var builtinPublicProcedures = []string{
	adminv1connect.AuthServiceLoginProcedure,
	adminv1connect.AuthServiceRefreshTokenProcedure,
}

var builtinPermissionEntries = []PermissionEntry{
	{Procedure: adminv1connect.AuthServiceLogoutProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.AuthServiceGetUserInfoProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.AuthServiceChangePasswordProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.DictServiceListDictTypesProcedure, Permission: "system:dict:list"},
	{Procedure: adminv1connect.DictServiceCreateDictTypeProcedure, Permission: "system:dict:add"},
	{Procedure: adminv1connect.DictServiceGetDictTypeProcedure, Permission: "system:dict:list"},
	{Procedure: adminv1connect.DictServiceUpdateDictTypeProcedure, Permission: "system:dict:edit"},
	{Procedure: adminv1connect.DictServiceDeleteDictTypeProcedure, Permission: "system:dict:remove"},
	{Procedure: adminv1connect.DictServiceBatchDeleteDictTypesProcedure, Permission: "system:dict:remove"},
	{Procedure: adminv1connect.DictServiceListDictDataProcedure, Permission: "system:dict:list"},
	{Procedure: adminv1connect.DictServiceCreateDictDataProcedure, Permission: "system:dict:add"},
	{Procedure: adminv1connect.DictServiceGetDictDataProcedure, Permission: "system:dict:list"},
	{Procedure: adminv1connect.DictServiceUpdateDictDataProcedure, Permission: "system:dict:edit"},
	{Procedure: adminv1connect.DictServiceDeleteDictDataProcedure, Permission: "system:dict:remove"},
	{Procedure: adminv1connect.DictServiceBatchDeleteDictDataProcedure, Permission: "system:dict:remove"},
	{Procedure: adminv1connect.ConfigServiceListConfigsProcedure, Permission: "system:config:list"},
	{Procedure: adminv1connect.ConfigServiceCreateConfigProcedure, Permission: "system:config:add"},
	{Procedure: adminv1connect.ConfigServiceGetConfigProcedure, Permission: "system:config:list"},
	{Procedure: adminv1connect.ConfigServiceUpdateConfigProcedure, Permission: "system:config:edit"},
	{Procedure: adminv1connect.ConfigServiceDeleteConfigProcedure, Permission: "system:config:remove"},
	{Procedure: adminv1connect.ConfigServiceBatchDeleteConfigsProcedure, Permission: "system:config:remove"},
	{Procedure: adminv1connect.LogServiceListOperLogsProcedure, Permission: "system:operlog:list"},
	{Procedure: adminv1connect.LogServiceCleanOperLogsProcedure, Permission: "system:operlog:remove"},
	{Procedure: adminv1connect.LogServiceListLoginLogsProcedure, Permission: "system:logininfor:list"},
	{Procedure: adminv1connect.LogServiceCleanLoginLogsProcedure, Permission: "system:logininfor:remove"},
	{Procedure: adminv1connect.UploadServiceUploadFileProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.UploadServiceUploadAvatarProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.CommonExportServiceBatchDeleteProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.CommonExportServiceExportExcelProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.CommonExportServiceImportExcelProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.ProfileServiceGetProfileProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.ProfileServiceUpdateProfileProcedure, Permission: AuthSentinel},
	{Procedure: adminv1connect.DashboardServiceOverviewProcedure, Permission: AuthSentinel},
}
