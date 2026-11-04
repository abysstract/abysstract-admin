// Package interceptor 提供 Connect 拦截器。
package interceptor

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/identityctx"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

var (
	errUnauthenticated         = errors.New("缺少或格式错误的 Authorization 头")
	errAuthRegistryUnavailable = errors.New("权限注册表不可用或尚未冻结")
)

// ErrTokenRevoked 旧 token 因 token_version 不匹配被拒绝（改密/登出后）。
// 单独导出便于上层测试断言。
var ErrTokenRevoked = errors.New("token 已失效，请重新登录")

// ErrPwdResetRequired 触发首次改密门禁。
var ErrPwdResetRequired = errors.New("需要修改初始密码后才能访问")

type ctxKey int

const claimsKey ctxKey = iota

// TokenVersionLoader 返回用户当前的 token_version（即 claims.Ver 应等于的值）。
// 若 user 不存在/已删除，返回错误；由拦截器转 Unauthenticated。
type TokenVersionLoader func(ctx context.Context, userID int64) (int64, error)

// PwdResetRequiredLoader 返回用户的 pwd_reset_required（0=已改密 / 1=需改密）。
type PwdResetRequiredLoader func(ctx context.Context, userID int64) (bool, error)

// NewAuthInterceptor 校验 Bearer token；公开接口放行，其余注入 Claims 到 context。
// 若提供了 verLoader，会比对 claims.Ver == user.token_version，不等则视为已吊销。
// 若提供了 resetLoader 且返回 true（首次登录/被重置后），拦截除「改密」「登出」外的请求。
//
// registry 必须是 bootstrap 后已经 Freeze 的同一实例；nil 或未冻结时 fail-closed。
func NewAuthInterceptor(
	registry *PermRegistry,
	mgr *jwt.Manager,
	verLoader TokenVersionLoader,
	resetLoader PwdResetRequiredLoader,
) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			proc := spec.Procedure

			// Registry 未冻结或不可用时 fail-closed，避免 bootstrap 漏登记导致默认放行。
			public, _, frozen := registry.Lookup(proc)
			if !frozen {
				return connect.NewError(connect.CodeUnauthenticated, errAuthRegistryUnavailable.Error()).WithCause(errAuthRegistryUnavailable)
			}
			// 公开过程（Login / RefreshToken）放行：不需要 Authorization。
			if public {
				return next(ctx, spec, stream)
			}

			info, _ := connect.CallInfoForServerContext(ctx)
			claims, err := VerifyBearerToken(ctx, info.RequestHeader().Get("Authorization"), mgr, verLoader)
			if err != nil {
				return connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
			}
			if err := EnforcePwdReset(ctx, proc, claims, resetLoader); err != nil {
				if errors.Is(err, ErrPwdResetRequired) {
					return connect.NewError(connect.CodePermissionDenied, err.Error()).WithCause(err)
				}
				return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
			}
			return next(WithClaims(ctx, claims), spec, stream)
		}
	}
}

// isPwdResetGateAllowed 返回该过程在「需改密」状态下是否仍放行。
// 放行：ChangePassword（完成改密）、Logout（避免无法登出）。
func isPwdResetGateAllowed(proc string) bool {
	switch proc {
	case "/abysstract.admin.v1.AuthService/ChangePassword",
		"/abysstract.admin.v1.AuthService/Logout",
		"/abysstract.admin.v1.AuthService/GetUserInfo":
		return true
	}
	return false
}

// VerifyBearerToken parses a Bearer token and checks token_version.
// Callers map the returned sentinel errors onto Connect or HTTP status codes.
func VerifyBearerToken(
	ctx context.Context,
	authorizationHeader string,
	mgr *jwt.Manager,
	verLoader TokenVersionLoader,
) (*jwt.Claims, error) {
	token := strings.TrimPrefix(authorizationHeader, "Bearer ")
	if token == "" || token == authorizationHeader {
		return nil, errUnauthenticated
	}
	if mgr == nil {
		return nil, errUnauthenticated
	}
	claims, err := mgr.Parse(token)
	if err != nil {
		return nil, err
	}
	if verLoader != nil {
		curVer, err := verLoader(ctx, claims.UserID)
		if err != nil {
			return nil, err
		}
		if claims.Ver != curVer {
			return nil, ErrTokenRevoked
		}
	}
	return claims, nil
}

// EnforcePwdReset rejects protected procedures when the user must change an
// initial password. ChangePassword, Logout, and GetUserInfo remain allowed.
func EnforcePwdReset(
	ctx context.Context,
	procedure string,
	claims *jwt.Claims,
	resetLoader PwdResetRequiredLoader,
) error {
	if resetLoader == nil || claims == nil {
		return nil
	}
	required, err := resetLoader(ctx, claims.UserID)
	if err != nil {
		return err
	}
	if required && !isPwdResetGateAllowed(procedure) {
		return ErrPwdResetRequired
	}
	return nil
}

// ClaimsFromContext 从 context 取出已鉴权用户信息。
func ClaimsFromContext(ctx context.Context) (*jwt.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*jwt.Claims)
	return c, ok
}

// WithClaims 把 Claims 注入 context。供 service 测试模拟拦截器上下文使用。
// 业务代码不应直接调用（应通过拦截器链）。系统测试入口同步写入公共 identity。
func WithClaims(ctx context.Context, c *jwt.Claims) context.Context {
	ctx = context.WithValue(ctx, claimsKey, c)
	if c != nil {
		ctx = identityctx.Store(ctx, c.UserID, c.Username)
	}
	return ctx
}
