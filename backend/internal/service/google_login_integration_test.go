//go:build integration

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/googleid"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// fakeGoogle returns canned claims for tokens it knows, standing in for
// googleid.Verifier (which has its own tests).
type fakeGoogle struct {
	tokens map[string]googleid.Claims
	err    error
}

func (f *fakeGoogle) ClientID() string { return "client-id" }

func (f *fakeGoogle) Verify(_ context.Context, token string) (googleid.Claims, error) {
	if f.err != nil {
		return googleid.Claims{}, f.err
	}
	c, ok := f.tokens[token]
	if !ok {
		return googleid.Claims{}, googleid.ErrInvalidToken
	}
	return c, nil
}

func TestLoginWithGoogle(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	r := repo.New(gdb)
	mgr := jwt.NewManager("test-secret-google", time.Hour)
	svc := service.NewAuthService(r,
		repo.NewLoginSecurity(gdb, 5, 900),
		repo.NewRefreshTokenRepo(gdb),
		mgr, time.Hour, false, 7*24*time.Hour, false)
	google := &fakeGoogle{tokens: map[string]googleid.Claims{}}

	mkUser := func(name, email string) int64 {
		t.Helper()
		u, err := createUserForTest(r, name, "Passw0rd!x")
		if err != nil {
			t.Fatal(err)
		}
		if err := gdb.Exec("UPDATE sys_user SET email = ? WHERE id = ?", email, u.ID).Error; err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	login := func(token string) (*adminv1.LoginResponse, error) {
		return svc.LoginWithGoogle(context.Background(), &adminv1.LoginWithGoogleRequest{IdToken: token})
	}
	wantCode := func(t *testing.T, err error, code connect.Code) {
		t.Helper()
		var cerr *connect.Error
		if !errors.As(err, &cerr) || cerr.Code() != code {
			t.Fatalf("err = %v, want code %v", err, code)
		}
	}

	t.Run("disabled_without_verifier", func(t *testing.T) {
		_, err := login("anything")
		wantCode(t, err, connect.CodeFailedPrecondition)
		opts, _ := svc.GetLoginOptions(context.Background(), &adminv1.GetLoginOptionsRequest{})
		if opts.GetGoogleClientId() != "" {
			t.Fatalf("client id = %q, want empty", opts.GetGoogleClientId())
		}
	})

	svc.UseGoogleLogin(google)

	t.Run("options_expose_client_id", func(t *testing.T) {
		opts, err := svc.GetLoginOptions(context.Background(), &adminv1.GetLoginOptionsRequest{})
		if err != nil || opts.GetGoogleClientId() != "client-id" {
			t.Fatalf("opts = %v, err = %v", opts, err)
		}
	})

	aliceID := mkUser("alice", "Alice@Corp.com")

	t.Run("first_login_binds_by_email_case_insensitive", func(t *testing.T) {
		google.tokens["alice-1"] = googleid.Claims{Subject: "g-alice", Email: "alice@corp.com", EmailVerified: true}
		resp, err := login("alice-1")
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		claims, err := mgr.Parse(resp.GetToken())
		if err != nil || claims.UserID != aliceID || claims.Username != "alice" {
			t.Fatalf("claims = %+v, err = %v", claims, err)
		}
	})

	t.Run("bound_account_survives_email_change", func(t *testing.T) {
		if err := gdb.Exec("UPDATE sys_user SET email = 'new@corp.com' WHERE id = ?", aliceID).Error; err != nil {
			t.Fatal(err)
		}
		google.tokens["alice-2"] = googleid.Claims{Subject: "g-alice", Email: "renamed@corp.com", EmailVerified: true}
		resp, err := login("alice-2")
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		if claims, _ := mgr.Parse(resp.GetToken()); claims.UserID != aliceID {
			t.Fatalf("user = %d, want %d", claims.UserID, aliceID)
		}
	})

	t.Run("other_google_account_with_same_email_is_refused", func(t *testing.T) {
		// 同一用户已绑定别的 Google 账号：即使邮箱现在对得上，也不能再绑一个。
		if err := gdb.Exec("UPDATE sys_user SET email = 'alice@corp.com' WHERE id = ?", aliceID).Error; err != nil {
			t.Fatal(err)
		}
		google.tokens["mallory"] = googleid.Claims{Subject: "g-mallory", Email: "alice@corp.com", EmailVerified: true}
		_, err := login("mallory")
		wantCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("unknown_email_is_refused_and_not_created", func(t *testing.T) {
		google.tokens["nobody"] = googleid.Claims{Subject: "g-nobody", Email: "nobody@corp.com", EmailVerified: true}
		_, err := login("nobody")
		wantCode(t, err, connect.CodePermissionDenied)
		var n int64
		gdb.Raw("SELECT COUNT(*) FROM sys_user WHERE email = 'nobody@corp.com'").Scan(&n)
		if n != 0 {
			t.Fatal("login must not create users")
		}
	})

	t.Run("ambiguous_email_is_refused", func(t *testing.T) {
		mkUser("dup1", "dup@corp.com")
		mkUser("dup2", "DUP@corp.com")
		google.tokens["dup"] = googleid.Claims{Subject: "g-dup", Email: "dup@corp.com", EmailVerified: true}
		_, err := login("dup")
		wantCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("deleted_user_is_not_matched", func(t *testing.T) {
		id := mkUser("gone", "gone@corp.com")
		if err := gdb.Exec("UPDATE sys_user SET del_flag = 1 WHERE id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		google.tokens["gone"] = googleid.Claims{Subject: "g-gone", Email: "gone@corp.com", EmailVerified: true}
		_, err := login("gone")
		wantCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("disabled_user_is_refused", func(t *testing.T) {
		id := mkUser("off", "off@corp.com")
		if err := gdb.Exec("UPDATE sys_user SET status = 2 WHERE id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		google.tokens["off"] = googleid.Claims{Subject: "g-off", Email: "off@corp.com", EmailVerified: true}
		_, err := login("off")
		wantCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("invalid_token_is_unauthenticated", func(t *testing.T) {
		_, err := login("garbage")
		wantCode(t, err, connect.CodeUnauthenticated)
	})

	t.Run("unreachable_google_is_unavailable", func(t *testing.T) {
		google.err = googleid.ErrUnavailable
		defer func() { google.err = nil }()
		_, err := login("alice-1")
		wantCode(t, err, connect.CodeUnavailable)
	})

	t.Run("login_log_records_success_and_failure", func(t *testing.T) {
		var ok, failed int64
		gdb.Raw("SELECT COUNT(*) FROM sys_login_log WHERE username = 'alice' AND status = 1 AND msg LIKE '%Google%'").Scan(&ok)
		gdb.Raw("SELECT COUNT(*) FROM sys_login_log WHERE status = 2 AND msg LIKE 'Google 登录失败%'").Scan(&failed)
		if ok < 2 || failed < 3 {
			t.Fatalf("success rows = %d, failure rows = %d", ok, failed)
		}
	})
}
