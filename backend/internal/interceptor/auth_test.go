package interceptor

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

// headerRequest 提供鉴权测试使用的过程和请求头。
type headerRequest struct {
	procedure string
	header    http.Header
}

func (m *headerRequest) Spec() connect.Spec { return connect.Spec{Procedure: m.procedure} }
func (m *headerRequest) Header() http.Header {
	if m.header == nil {
		m.header = http.Header{}
	}
	return m.header
}

// Feature: open-source-readiness, Property 9: 会话失效（token 版本号）
//
// 不变量：
//  1. claims.Ver == loader 返回的 curVer → 请求放行（next 被调用）。
//  2. claims.Ver != curVer → 请求拒绝（ErrTokenRevoked）。
//  3. loader 返回错误 → 请求拒绝（Unauthenticated，原因 = loader 错误）。
//  4. verLoader == nil 时跳过该检查（向后兼容）。
//
// 通过改 curVer 模拟改密/登出后 token_version 递增 → 旧 token 失效。
func TestTokenVersionRevocation(t *testing.T) {
	registry := newFrozenTestRegistry(t)

	mgr := jwt.NewManager("test-secret", time.Hour)
	mgrGenerate := func(userID int64, ver int64) string {
		tok, _, err := mgr.Generate(userID, "alice", ver)
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		return tok
	}

	cases := []struct {
		name     string
		claimVer int64
		curVer   int64
		loaderOK bool
		loader   func() TokenVersionLoader
		wantNext bool
		wantErr  error
	}{
		{
			name:     "version_match_passes",
			claimVer: 5, curVer: 5,
			loader:   func() TokenVersionLoader { return stubTokenVerLoader(5, nil) },
			wantNext: true,
		},
		{
			name:     "version_mismatch_revokes",
			claimVer: 5, curVer: 6,
			loader:   func() TokenVersionLoader { return stubTokenVerLoader(6, nil) },
			wantNext: false,
			wantErr:  ErrTokenRevoked,
		},
		{
			name:     "loader_error_rejects",
			claimVer: 5,
			loader:   func() TokenVersionLoader { return stubTokenVerLoader(0, errors.New("db error")) },
			wantNext: false,
		},
		{
			name:     "nil_loader_skips_check",
			claimVer: 5,
			loader:   func() TokenVersionLoader { return nil },
			wantNext: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token := mgrGenerate(1, c.claimVer)
			it := NewAuthInterceptor(registry, mgr, c.loader(), nil)

			nextCalled := false
			next := connect.ServerFunc(func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				nextCalled = true
				return nil
			})
			req := &headerRequest{procedure: testProcedure}
			req.Header().Set("Authorization", "Bearer "+token)

			err := req.call(it, next)
			if nextCalled != c.wantNext {
				t.Errorf("nextCalled = %v, want %v (err=%v)", nextCalled, c.wantNext, err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

// Feature: open-source-readiness, Property 9 扩展：首次改密门禁
//
// 不变量：当 resetLoader 返回 true 时，只有 ChangePassword/Logout/GetUserInfo 放行，
// 其余过程（含带 AuthSentinel 的）一律拒绝（ErrPwdResetRequired）。
func TestPwdResetGate(t *testing.T) {
	registry := newFrozenTestRegistry(t)

	mgr := jwt.NewManager("test-secret", time.Hour)
	token, _, err := mgr.Generate(1, "alice", 0)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	gateProcs := []string{
		"/abysstract.admin.v1.AuthService/ChangePassword",
		"/abysstract.admin.v1.AuthService/Logout",
		"/abysstract.admin.v1.AuthService/GetUserInfo",
	}
	blockedProcs := []string{
		"/abysstract.admin.v1.UserService/ListUsers",
		"/abysstract.admin.v1.RoleService/CreateRole",
	}
	verLoader := stubTokenVerLoader(0, nil)
	resetLoaderTrue := stubPwdResetLoader(true, nil)
	it := NewAuthInterceptor(registry, mgr, verLoader, resetLoaderTrue)

	for _, proc := range gateProcs {
		req := &headerRequest{procedure: proc}
		req.Header().Set("Authorization", "Bearer "+token)
		nextCalled := false
		next := connect.ServerFunc(func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			nextCalled = true
			return nil
		})
		if err := req.call(it, next); err != nil {
			t.Errorf("proc %s should pass gate, got err: %v", proc, err)
		}
		if !nextCalled {
			t.Errorf("proc %s next not called", proc)
		}
	}
	for _, proc := range blockedProcs {
		req := &headerRequest{procedure: proc}
		req.Header().Set("Authorization", "Bearer "+token)
		next := connect.ServerFunc(func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			t.Errorf("proc %s should be blocked but next called", proc)
			return nil
		})
		err := req.call(it, next)
		if !errors.Is(err, ErrPwdResetRequired) {
			t.Errorf("proc %s err = %v, want ErrPwdResetRequired", proc, err)
		}
	}
}

func stubTokenVerLoader(v int64, err error) TokenVersionLoader {
	return func(_ context.Context, _ int64) (int64, error) { return v, err }
}

func stubPwdResetLoader(req bool, err error) PwdResetRequiredLoader {
	return func(_ context.Context, _ int64) (bool, error) { return req, err }
}

func newFrozenTestRegistry(t *testing.T, publicProcedures ...string) *PermRegistry {
	t.Helper()
	registry := NewPermRegistry()
	if err := registry.RegisterPublic(publicProcedures...); err != nil {
		t.Fatalf("register public procedures: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return registry
}

func (m *headerRequest) call(interceptor connect.ServerInterceptor, next connect.ServerFunc) error {
	server := connect.NewServer(interceptor)
	server.Register(connect.Method{Spec: m.Spec(), Handler: next})
	info := &connect.CallInfo{}
	info.RequestHeader().Set("Authorization", m.Header().Get("Authorization"))
	return server.Call(context.Background(), m.procedure, info, nil)
}
