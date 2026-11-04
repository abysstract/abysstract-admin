package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/abysstract/abysstract-admin/backend/identity"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// AuthService 实现 adminv1connect.AuthServiceHandler。
type AuthService struct {
	repo         *repo.Repo
	loginSec     *repo.LoginSecurity
	refresh      *repo.RefreshTokenRepo
	jwt          *jwt.Manager
	jwtExpiresIn time.Duration
	// 刷新令牌
	refreshEnabled   bool
	refreshExpiresIn time.Duration
	trustProxy       bool
	now              func() time.Time
}

// NewAuthService 构造 AuthService。若 refreshEnabled=false，RefreshToken/refresh_token 字段被禁用。
func NewAuthService(
	r *repo.Repo,
	loginSec *repo.LoginSecurity,
	refresh *repo.RefreshTokenRepo,
	mgr *jwt.Manager,
	jwtExpiresIn time.Duration,
	refreshEnabled bool,
	refreshExpiresIn time.Duration,
	trustProxy bool,
) *AuthService {
	return &AuthService{
		repo:             r,
		loginSec:         loginSec,
		refresh:          refresh,
		jwt:              mgr,
		jwtExpiresIn:     jwtExpiresIn,
		refreshEnabled:   refreshEnabled,
		refreshExpiresIn: refreshExpiresIn,
		trustProxy:       trustProxy,
		now:              time.Now,
	}
}

// loginIP 解析登录来源 IP。仅在信任反向代理时采用 X-Forwarded-For 最左地址，
// 否则使用直连对端地址（去端口），避免外部伪造 XFF 头污染登录审计日志。
func (s *AuthService) loginIP(header *connect.Header, peerAddr string) string {
	if s.trustProxy {
		if xff := header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	if host, _, err := net.SplitHostPort(peerAddr); err == nil {
		return host
	}
	return peerAddr
}

// Login 验证密码 → 检查账号锁定 → 签发 JWT 与可选 refresh_token。
// 失败时通过 LoginSecurity 记录失败计数，达到阈值触发锁定（需求 6.6）。
func (s *AuthService) Login(
	ctx context.Context, req *adminv1.LoginRequest,
) (*adminv1.LoginResponse, error) {
	ip := ""
	if info, ok := connect.CallInfoForServerContext(ctx); ok {
		ip = s.loginIP(info.RequestHeader(), info.PeerAddr)
	}
	username := identity.ASCIICaseKey(req.Username)
	now := s.now()

	// 1) 锁定检查（需求 6.7）：先于密码校验，避免锁定状态下泄漏任何信息。
	if s.loginSec != nil {
		locked, until, err := s.loginSec.IsLocked(ctx, username, now)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		if locked {
			remaining := time.Until(until).Round(time.Second).String()
			_ = s.repo.AddLoginLog(ctx, &model.SysLoginLog{Username: username, Status: 2, IP: ip, Msg: "登录失败：账号锁定中"})
			return nil, connect.NewError(connect.CodePermissionDenied, "账号已锁定，剩余 "+remaining)
		}
	}

	u, err := s.repo.VerifyPassword(ctx, username, req.Password)
	if err != nil {
		// 2) 失败计数 +1（需求 6.6）。
		// 计数失败视为基础设施异常，拒绝登录而不是放行：避免攻击者通过打挂
		// sys_login_attempt 让计数永远写不进而在阈值之外反复尝试。
		if s.loginSec != nil {
			if _, _, ferr := s.loginSec.RecordFailure(ctx, username, now); ferr != nil {
				_ = s.repo.AddLoginLog(ctx, &model.SysLoginLog{Username: username, Status: 2, IP: ip, Msg: "登录失败：计数器异常"})
				return nil, connect.NewError(connect.CodeInternal, ferr.Error()).WithCause(ferr)
			}
		}
		_ = s.repo.AddLoginLog(ctx, &model.SysLoginLog{Username: username, Status: 2, IP: ip, Msg: "登录失败：用户名或密码错误"})
		return nil, connect.NewError(connect.CodeUnauthenticated, "用户名或密码错误")
	}
	if u.Status != int16(adminv1.Status_STATUS_ENABLED) {
		_ = s.repo.AddLoginLog(ctx, &model.SysLoginLog{Username: username, Status: 2, IP: ip, Msg: "登录失败：账号已停用"})
		return nil, connect.NewError(connect.CodePermissionDenied, errAccountDisabled.Error()).WithCause(errAccountDisabled)
	}

	// 3) 成功：重置失败计数（需求 6.7）。
	if s.loginSec != nil {
		_ = s.loginSec.Reset(ctx, username)
	}

	token, expiresIn, err := s.jwt.Generate(u.ID, u.Username, int64(u.TokenVersion))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	resp := &adminv1.LoginResponse{Token: token, ExpiresIn: expiresIn}

	// 4) 可选：签发 refresh_token（需求 6.10）。
	// 绑定当前 user.token_version：RefreshToken RPC 校验 rec.token_version == user.token_version，
	// 使改密/重置密码后即使有漏网的 refresh 记录也会因版本不匹配被拒绝。
	if s.refreshEnabled && s.refresh != nil {
		raw, err := generateRefreshToken()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		expiresAt := now.Add(s.refreshExpiresIn)
		if err := s.refresh.Create(ctx, u.ID, hashRefreshToken(raw), expiresAt, u.TokenVersion); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		resp.RefreshToken = raw
	}

	_ = s.repo.AddLoginLog(ctx, &model.SysLoginLog{Username: username, Status: 1, IP: ip, Msg: "登录成功"})
	return resp, nil
}

// Logout 吊销该会话对应的 token_version（让所有同 ver 的 token 失效）。
// 由于无状态 JWT 不携带 token jti，这里以「递增 user.token_version」实现登出失效。
// 若启用 refresh token，同时吊销该用户所有 refresh token。
func (s *AuthService) Logout(
	ctx context.Context, _ *adminv1.LogoutRequest,
) (*adminv1.LogoutResponse, error) {
	claims, ok := interceptor.ClaimsFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	if err := s.repo.IncrementTokenVersion(ctx, claims.UserID); err != nil {
		return nil, toConnectError(err)
	}
	if s.refreshEnabled && s.refresh != nil {
		_ = s.refresh.RevokeAllForUser(ctx, claims.UserID)
	}
	return &adminv1.LogoutResponse{}, nil
}

// ChangePassword 校验旧密码 + 复杂度，更新密码哈希并递增 token_version。
// 首次登录需改密的用户只能调用此方法（拦截器门禁）。
func (s *AuthService) ChangePassword(
	ctx context.Context, req *adminv1.ChangePasswordRequest,
) (*adminv1.ChangePasswordResponse, error) {
	claims, ok := interceptor.ClaimsFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}

	if req.GetOldPassword() == req.GetNewPassword() {
		return nil, connect.NewError(connect.CodeInvalidArgument, "新密码不能与旧密码相同")
	}
	if err := validatePasswordPolicy(req.GetNewPassword()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	// 校验旧密码（防止会话被劫后改密）。
	u, err := s.repo.GetUser(ctx, claims.UserID)
	if err != nil {
		return nil, toConnectError(err)
	}
	if !verifyPassword(u.Password, req.GetOldPassword()) {
		return nil, connect.NewError(connect.CodeUnauthenticated, "旧密码不正确")
	}

	// hash 新密码并写入。
	newHash, err := hashPassword(req.GetNewPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	// 密码、会话版本和刷新令牌状态在同一事务内更新，任一步骤失败时共同回滚。
	var refresh *repo.RefreshTokenRepo
	if s.refreshEnabled {
		refresh = s.refresh
	}
	if err := s.repo.ChangeOwnPasswordAndRevokeRefresh(ctx, claims.UserID, newHash, refresh); err != nil {
		return nil, toConnectError(err)
	}

	return &adminv1.ChangePasswordResponse{}, nil
}

// RefreshToken 用 refresh_token 换新的 access token。
// 未启用刷新令牌特性时返回 Unimplemented。
// 校验：存在 / 未吊销 / 未过期 / 用户仍启用。轮转：每次成功后吊销旧 token，签发新 token。
func (s *AuthService) RefreshToken(
	ctx context.Context, req *adminv1.RefreshTokenRequest,
) (*adminv1.RefreshTokenResponse, error) {
	if !s.refreshEnabled {
		return nil, connect.NewError(connect.CodeUnimplemented, "刷新令牌特性未启用")
	}
	raw := req.GetRefreshToken()
	if raw == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, "缺少 refresh_token")
	}
	now := s.now()
	rec, err := s.refresh.LookupActive(ctx, hashRefreshToken(raw), now)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
	}

	var response *adminv1.RefreshTokenResponse
	err = s.repo.InTransaction(ctx, func(tx *repo.Repo) error {
		// Account writes take the user lock before revoking its refresh tokens.
		u, err := tx.GetUserForUpdate(ctx, rec.UserID)
		if err != nil {
			return toConnectError(err)
		}
		refresh := repo.NewRefreshTokenRepo(tx.DB())
		active, err := refresh.LookupActive(ctx, hashRefreshToken(raw), now)
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
		}
		if u.Status != int16(adminv1.Status_STATUS_ENABLED) {
			return connect.NewError(connect.CodePermissionDenied, errAccountDisabled.Error()).WithCause(errAccountDisabled)
		}
		if active.TokenVersion != u.TokenVersion {
			return connect.NewError(connect.CodeUnauthenticated, "refresh token 已失效，请重新登录")
		}
		if err := refresh.Revoke(ctx, hashRefreshToken(raw)); err != nil {
			if errors.Is(err, repo.ErrRefreshTokenRevoked) {
				return connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
			}
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		newRaw, err := generateRefreshToken()
		if err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		if err := refresh.Create(ctx, u.ID, hashRefreshToken(newRaw), now.Add(s.refreshExpiresIn), u.TokenVersion); err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		token, expiresIn, err := s.jwt.Generate(u.ID, u.Username, int64(u.TokenVersion))
		if err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		response = &adminv1.RefreshTokenResponse{Token: token, ExpiresIn: expiresIn, RefreshToken: newRaw}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *AuthService) GetUserInfo(
	ctx context.Context, _ *adminv1.GetUserInfoRequest,
) (*adminv1.GetUserInfoResponse, error) {
	claims, ok := interceptor.ClaimsFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	u, err := s.repo.GetUser(ctx, claims.UserID)
	if err != nil {
		return nil, toConnectError(err)
	}
	roleIDs, err := s.repo.RoleIDsOf(ctx, u.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	roles, err := s.repo.RoleKeysOf(ctx, u.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	perms, err := s.repo.PermsOf(ctx, u.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	menus, err := s.repo.MenusOf(ctx, u.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.GetUserInfoResponse{
		User:             toProtoUser(u, roleIDs),
		Roles:            roles,
		Permissions:      perms,
		Menus:            buildMenuTree(menus),
		PwdResetRequired: u.PwdResetRequired == 1,
	}, nil
}

// generateRefreshToken 生成 32 字节（base64url 43 字符）的随机刷新令牌。
// 用 crypto/rand 保证不可预测。
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
