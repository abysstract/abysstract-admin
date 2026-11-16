//go:build integration

package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/connect/v2/connectproto"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/captcha"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// fakeChallenger answers "42"; the token "boom" simulates an unreachable provider.
type fakeChallenger struct{}

func (fakeChallenger) Challenge(context.Context) (captcha.Challenge, error) {
	return captcha.Challenge{Kind: "fake", Payload: `{}`, Answer: []byte("42")}, nil
}

func (fakeChallenger) Verify(_ context.Context, sub captcha.Submission) error {
	switch {
	case sub.Token == "boom":
		return errors.New("provider down")
	case string(sub.Answer) == "42" && sub.Token == "42":
		return nil
	default:
		return captcha.ErrInvalid
	}
}

type guardEnv struct {
	gdb    *gorm.DB
	client adminv1connect.AuthServiceClient
}

func newGuardEnv(t *testing.T, verifier captcha.Verifier) guardEnv {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.New(gdb)
	svc := service.NewAuthService(r, repo.NewLoginSecurity(gdb, 20, 900), repo.NewRefreshTokenRepo(gdb),
		jwt.NewManager("login-guard-test", time.Hour), time.Hour, false, 7*24*time.Hour, false).
		UseProxyPolicy(accesslog.ProxyPolicy{Trust: true}).
		UseLoginGuard(&service.LoginGuard{
			Captcha:        verifier,
			IP:             repo.NewLoginIPGuard(gdb, 900),
			Used:           repo.NewCaptchaUsedRepo(gdb),
			SealKey:        service.CaptchaSealKey("login-guard-test"),
			CaptchaAfter:   2,
			IPCaptchaAfter: 4,
			IPMaxFailures:  3,
		})
	server := connect.NewServer()
	adminv1connect.RegisterAuthServiceHandler(server, svc)
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	srv := httptest.NewUnstartedServer(h2c.NewHandler(mux, &http2.Server{}))
	srv.EnableHTTP2 = true
	srv.Start()
	t.Cleanup(srv.Close)
	client := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))
	return guardEnv{gdb: gdb, client: client}
}

// fromIP sends the request as if a trusted proxy reported ip.
func fromIP(ip string) context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("X-Forwarded-For", ip)
	return ctx
}

func challengeOf(t *testing.T, err error) (connect.Code, *adminv1.LoginChallenge) {
	t.Helper()
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("want connect error, got %v", err)
	}
	for _, d := range cerr.Details() {
		msg, derr := connectproto.UnmarshalErrorDetail(d)
		if derr != nil {
			t.Fatal(derr)
		}
		if lc, ok := msg.(*adminv1.LoginChallenge); ok {
			return cerr.Code(), lc
		}
	}
	return cerr.Code(), nil
}

func TestLoginCaptchaAfterAccountFailures(t *testing.T) {
	env := newGuardEnv(t, fakeChallenger{})
	ctx := fromIP("198.51.100.1")
	wrong := &adminv1.LoginRequest{Username: "admin", Password: "wrong"}

	_, err := env.client.Login(ctx, wrong)
	if code, lc := challengeOf(t, err); code != connect.CodeUnauthenticated || lc != nil {
		t.Fatalf("1st failure: code=%v challenge=%v", code, lc)
	}
	_, err = env.client.Login(ctx, wrong)
	if code, lc := challengeOf(t, err); code != connect.CodeUnauthenticated || !lc.GetCaptchaRequired() || lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_UNSPECIFIED {
		t.Fatalf("2nd failure must announce the captcha: code=%v challenge=%v", code, lc)
	}

	// Without a captcha even the right password is refused, and the attempt is not counted.
	_, err = env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if _, lc := challengeOf(t, err); lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_MISSING {
		t.Fatalf("missing captcha: %v", err)
	}

	ch, err := env.client.GetCaptcha(ctx, &adminv1.GetCaptchaRequest{})
	if err != nil || ch.GetKind() != "fake" || ch.GetId() == "" || ch.GetExpiresIn() <= 0 {
		t.Fatalf("GetCaptcha = %v, %v", ch, err)
	}
	_, err = env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123", CaptchaId: ch.GetId(), Captcha: "7"})
	if _, lc := challengeOf(t, err); lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_INVALID {
		t.Fatalf("wrong answer: %v", err)
	}
	// The challenge was used up by the wrong answer.
	_, err = env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123", CaptchaId: ch.GetId(), Captcha: "42"})
	if _, lc := challengeOf(t, err); lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_INVALID {
		t.Fatalf("replayed challenge: %v", err)
	}
	// A forged ID is rejected.
	_, err = env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123", CaptchaId: "forged", Captcha: "42"})
	if _, lc := challengeOf(t, err); lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_INVALID {
		t.Fatalf("forged challenge: %v", err)
	}
	// Provider failures fail closed.
	ch, _ = env.client.GetCaptcha(ctx, &adminv1.GetCaptchaRequest{})
	_, err = env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123", CaptchaId: ch.GetId(), Captcha: "boom"})
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("provider down: %v", err)
	}

	ch, _ = env.client.GetCaptcha(ctx, &adminv1.GetCaptchaRequest{})
	resp, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123", CaptchaId: ch.GetId(), Captcha: "42"})
	if err != nil || resp.GetToken() == "" {
		t.Fatalf("solved captcha: %v", err)
	}
	// Success resets the account count: no captcha needed any more.
	if _, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123"}); err != nil {
		t.Fatalf("after reset: %v", err)
	}
}

func TestLoginCaptchaAfterIPFailures(t *testing.T) {
	env := newGuardEnv(t, fakeChallenger{})
	ctx := fromIP("2001:db8:1:2::10")
	// Spread over many usernames: no account reaches its threshold.
	for i, name := range []string{"u1", "u2", "u3", "u4"} {
		_, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: name, Password: "x"})
		_, lc := challengeOf(t, err)
		if want := i == 3; lc.GetCaptchaRequired() != want {
			t.Fatalf("failure %d: captcha_required=%v", i+1, lc.GetCaptchaRequired())
		}
	}
	// Another address in the same /64 shares the count.
	_, err := env.client.Login(fromIP("2001:db8:1:2::99"), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if _, lc := challengeOf(t, err); lc.GetCaptchaError() != adminv1.CaptchaError_CAPTCHA_ERROR_MISSING {
		t.Fatalf("same /64: %v", err)
	}
	// A different address is unaffected.
	if _, err := env.client.Login(fromIP("198.51.100.2"), &adminv1.LoginRequest{Username: "admin", Password: "admin123"}); err != nil {
		t.Fatalf("other ip: %v", err)
	}
}

func TestLoginIPLimitWithoutCaptcha(t *testing.T) {
	env := newGuardEnv(t, nil)
	ctx := fromIP("203.0.113.5")
	for i := 0; i < 3; i++ {
		_, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: "spray", Password: "x"})
		if code, lc := challengeOf(t, err); code != connect.CodeUnauthenticated || lc != nil {
			t.Fatalf("failure %d: %v %v", i+1, code, lc)
		}
	}
	_, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("over the IP limit: %v", err)
	}
	if _, err := env.client.GetCaptcha(ctx, &adminv1.GetCaptchaRequest{}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("GetCaptcha without captcha: %v", err)
	}
	// The window ends: the count starts over.
	if err := env.gdb.Exec("UPDATE sys_login_ip_attempt SET window_until = ?", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := env.client.Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123"}); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}
