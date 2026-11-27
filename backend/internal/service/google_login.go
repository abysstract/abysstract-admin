package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect/v2"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/googleid"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// GoogleVerifier checks a Google ID token. *googleid.Verifier implements it.
type GoogleVerifier interface {
	ClientID() string
	Verify(ctx context.Context, idToken string) (googleid.Claims, error)
}

var (
	errGoogleDisabled    = errors.New("未启用 Google 登录")
	errGoogleInvalid     = errors.New("Google 登录失败，请重试")
	errGoogleNoAccount   = errors.New("该 Google 账号未关联后台用户，请联系管理员")
	errGoogleUnreachable = errors.New("无法连接 Google 验证服务，请稍后再试")
)

// UseGoogleLogin enables LoginWithGoogle. Call it before the service handles
// requests.
func (s *AuthService) UseGoogleLogin(v GoogleVerifier) *AuthService {
	s.google = v
	return s
}

// GetLoginOptions tells the login page which sign-in methods are available.
func (s *AuthService) GetLoginOptions(
	_ context.Context, _ *adminv1.GetLoginOptionsRequest,
) (*adminv1.GetLoginOptionsResponse, error) {
	resp := &adminv1.GetLoginOptionsResponse{}
	if s.google != nil {
		resp.GoogleClientId = s.google.ClientID()
	}
	return resp, nil
}

// LoginWithGoogle signs in with a Google ID token. Only accounts that already
// exist can sign in: the first time, the verified email picks the user and the
// Google account ID (sub) is bound to it; after that only the ID counts, so a
// changed email neither locks the user out nor lets someone else in.
func (s *AuthService) LoginWithGoogle(
	ctx context.Context, req *adminv1.LoginWithGoogleRequest,
) (*adminv1.LoginResponse, error) {
	if s.google == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errGoogleDisabled.Error())
	}
	ip := ""
	if info, ok := connect.CallInfoForServerContext(ctx); ok {
		ip = s.loginIP(info.RequestHeader(), info.PeerAddr)
	}
	now := s.now()

	claims, err := s.google.Verify(ctx, strings.TrimSpace(req.IdToken))
	if err != nil {
		if errors.Is(err, googleid.ErrUnavailable) {
			return nil, connect.NewError(connect.CodeUnavailable, errGoogleUnreachable.Error()).WithCause(err)
		}
		s.logLoginFailure(ctx, "", ip, "Google 登录失败：令牌无效")
		return nil, connect.NewError(connect.CodeUnauthenticated, errGoogleInvalid.Error()).WithCause(err)
	}

	u, err := s.googleUser(ctx, claims)
	if err != nil {
		var cerr *connect.Error
		if errors.As(err, &cerr) && cerr.Code() == connect.CodePermissionDenied {
			s.logLoginFailure(ctx, claims.Email, ip, "Google 登录失败："+cerr.Message())
		}
		return nil, err
	}
	if u.Status != int16(adminv1.Status_STATUS_ENABLED) {
		s.logLoginFailure(ctx, u.Username, ip, "登录失败：账号已停用")
		return nil, connect.NewError(connect.CodePermissionDenied, errAccountDisabled.Error()).WithCause(errAccountDisabled)
	}
	return s.startSession(ctx, u, ip, now, "登录成功（Google）")
}

// googleUser finds the user for a verified Google account: by the bound
// account ID, or on first sign-in by email, binding it. A PermissionDenied
// error means no user can be signed in for this account.
func (s *AuthService) googleUser(ctx context.Context, claims googleid.Claims) (*model.SysUser, error) {
	u, err := s.repo.UserByIdentity(ctx, repo.ProviderGoogle, claims.Subject)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	users, err := s.repo.UsersByEmail(ctx, claims.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	// 邮箱匹配不到、或同一邮箱对应多个用户时无法确定是谁，一律拒绝。
	if len(users) != 1 {
		return nil, connect.NewError(connect.CodePermissionDenied, errGoogleNoAccount.Error()).WithCause(errGoogleNoAccount)
	}
	u = &users[0]
	if err := s.repo.BindIdentity(ctx, u.ID, repo.ProviderGoogle, claims.Subject, u.Email); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 该用户已绑定另一个 Google 账号，或这个 Google 账号刚被并发绑定。
			return nil, connect.NewError(connect.CodePermissionDenied, errGoogleNoAccount.Error()).WithCause(errGoogleNoAccount)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return u, nil
}
