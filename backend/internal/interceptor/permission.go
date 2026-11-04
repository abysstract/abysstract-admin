package interceptor

import (
	"context"
	"fmt"

	"connectrpc.com/connect/v2"
)

// PermLoader 返回指定用户的权限标识集合。
type PermLoader func(ctx context.Context, userID int64) ([]string, error)

// NewPermissionInterceptor 在鉴权之后做按钮/操作权限校验。
// 必须在 NewAuthInterceptor 之后注册（依赖 context 中的 Claims）。
//
// 默认拒绝（fail-closed）：非公开过程若未在 Registry 中声明权限，则拒绝。
// 公开过程放行；声明 AuthSentinel 的过程仅需登录，不校验具体权限。
// registry 必须与 Auth interceptor 共用同一个已冻结实例。
func NewPermissionInterceptor(registry *PermRegistry, loader PermLoader) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			proc := spec.Procedure

			// 基础设施不可用或尚未 Freeze → fail-closed，拒绝一切请求。
			public, required, frozen := registry.Lookup(proc)
			if !frozen {
				return connect.NewError(connect.CodePermissionDenied, errInfraUnavailable.Error()).WithCause(errInfraUnavailable)
			}

			// 公开过程直接放行。
			if public {
				return next(ctx, spec, stream)
			}

			// 非公开过程必须声明权限，否则默认拒绝。
			if required == "" {
				return connect.NewError(connect.CodePermissionDenied, errNoPermDeclared.Error()).WithCause(errNoPermDeclared)
			}

			claims, cok := ClaimsFromContext(ctx)
			if !cok {
				return connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
			}

			// 哨兵 "@authenticated"：仅需登录，不校验权限。
			if required == AuthSentinel {
				return next(ctx, spec, stream)
			}

			perms, err := loader(ctx, claims.UserID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
			}
			if !hasPerm(perms, required) {
				return connect.Errorf(
					connect.CodePermissionDenied,
					"缺少权限: %s", required,
				)
			}
			return next(ctx, spec, stream)
		}
	}
}

// HasPerm 判断 perms 中是否包含 required（含 "*:*:*" 通配语义）。
// 导出供 service 层做资源级权限校验使用（通用 RPC 在 service 派发时按 resource 再校验）。
func HasPerm(perms []string, required string) bool {
	for _, p := range perms {
		if p == "*:*:*" || p == required {
			return true
		}
	}
	return false
}

func hasPerm(perms []string, required string) bool { return HasPerm(perms, required) }

var (
	errNoClaims         = fmt.Errorf("未获取到登录信息")
	errNoPermDeclared   = fmt.Errorf("过程未声明所需权限（默认拒绝）")
	errInfraUnavailable = fmt.Errorf("权限基础设施不可用")
)
