package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"

	"github.com/abysstract/abysstract-admin/backend/captcha"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

// captchaChallengeTTL is how long a GetCaptcha challenge can be answered.
const captchaChallengeTTL = 2 * time.Minute

// LoginGuard holds the per-IP counting and captcha settings of Login.
type LoginGuard struct {
	// Captcha is nil when no captcha is configured; Login then refuses an IP
	// outright after IPMaxFailures instead of asking for a captcha.
	Captcha captcha.Verifier
	IP      *repo.LoginIPGuard
	Used    *repo.CaptchaUsedRepo
	// SealKey encrypts challenge answers into the challenge ID, so the server
	// keeps no state per challenge.
	SealKey []byte

	CaptchaAfter   int // account failures before a captcha is required
	IPCaptchaAfter int // IP failures in the window before a captcha is required
	IPMaxFailures  int // IP failures in the window before Login refuses (no captcha only)
}

// UseLoginGuard enables per-IP counting and, with g.Captcha, the captcha.
// Call it before the service handles requests.
func (s *AuthService) UseLoginGuard(g *LoginGuard) *AuthService {
	s.guard = g
	return s
}

// UseProxyPolicy sets how the client IP is read from X-Forwarded-For.
// Call it before the service handles requests.
func (s *AuthService) UseProxyPolicy(p accesslog.ProxyPolicy) *AuthService {
	s.proxies = p
	return s
}

func (g *LoginGuard) captchaRequired(accountFails, ipFails int32) bool {
	if g == nil || g.Captcha == nil {
		return false
	}
	return int(accountFails) >= g.CaptchaAfter || int(ipFails) >= g.IPCaptchaAfter
}

// loginIPKey groups addresses the way they are handed out: one IPv4 address,
// or one IPv6 /64, which a single host usually controls entirely.
func loginIPKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.WithZone("").Prefix(64)
	if err != nil {
		return ip
	}
	return prefix.String()
}

// loginChallengeError attaches a LoginChallenge detail to err.
func loginChallengeError(err *connect.Error, detail *adminv1.LoginChallenge) *connect.Error {
	d, derr := connectproto.NewErrorDetail(detail)
	if derr != nil {
		return err
	}
	return err.WithDetail(d)
}

var errCaptchaUnavailable = errors.New("人机验证暂时不可用，请稍后再试")

// checkCaptcha verifies the captcha of a login that requires one. It returns
// a connect error ready for the client, and the login log message.
func (s *AuthService) checkCaptcha(ctx context.Context, req *adminv1.LoginRequest, ip string, now time.Time) (*connect.Error, string) {
	g := s.guard
	if req.GetCaptcha() == "" {
		return loginChallengeError(connect.NewError(connect.CodeUnauthenticated, "请完成人机验证"),
			&adminv1.LoginChallenge{CaptchaRequired: true, CaptchaError: adminv1.CaptchaError_CAPTCHA_ERROR_MISSING}), "登录失败：缺少验证码"
	}
	invalid := func() (*connect.Error, string) {
		return loginChallengeError(connect.NewError(connect.CodeUnauthenticated, "人机验证未通过，请重试"),
			&adminv1.LoginChallenge{CaptchaRequired: true, CaptchaError: adminv1.CaptchaError_CAPTCHA_ERROR_INVALID}), "登录失败：验证码错误"
	}
	var answer []byte
	if id := req.GetCaptchaId(); id != "" {
		opened, expires, err := openChallenge(g.SealKey, id, now)
		if err != nil {
			return invalid()
		}
		fresh, err := g.Used.MarkUsed(ctx, challengeUseKey(id), expires, now)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err), "登录失败：验证码记录异常"
		}
		if !fresh {
			return invalid()
		}
		answer = opened
	}
	err := g.Captcha.Verify(ctx, captcha.Submission{Token: req.GetCaptcha(), Answer: answer, RemoteIP: ip})
	switch {
	case err == nil:
		return nil, ""
	case errors.Is(err, captcha.ErrInvalid):
		return invalid()
	default:
		return connect.NewError(connect.CodeUnavailable, errCaptchaUnavailable.Error()).WithCause(err), "登录失败：验证服务不可用"
	}
}

// GetCaptcha returns a challenge for the login page.
func (s *AuthService) GetCaptcha(ctx context.Context, _ *adminv1.GetCaptchaRequest) (*adminv1.GetCaptchaResponse, error) {
	if s.guard == nil || s.guard.Captcha == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, "未启用验证码")
	}
	challenger, ok := s.guard.Captcha.(captcha.Challenger)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, "当前验证码由页面自行加载")
	}
	ch, err := challenger.Challenge(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errCaptchaUnavailable.Error()).WithCause(err)
	}
	resp := &adminv1.GetCaptchaResponse{Kind: ch.Kind, Payload: ch.Payload}
	if ch.Answer != nil {
		id, err := sealChallenge(s.guard.SealKey, ch.Answer, s.now().Add(captchaChallengeTTL))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		resp.Id = id
		resp.ExpiresIn = int64(captchaChallengeTTL / time.Second)
	}
	return resp, nil
}

// CaptchaSealKey derives the challenge sealing key from the JWT secret.
func CaptchaSealKey(jwtSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte("abysstract-admin/captcha/challenge"))
	return mac.Sum(nil)
}

type sealedChallenge struct {
	Answer  []byte `json:"a"`
	Expires int64  `json:"e"`
}

// sealChallenge encrypts the answer and expiry with AES-GCM; the result is
// the challenge ID the client sends back.
func sealChallenge(key, answer []byte, expires time.Time) (string, error) {
	aead, err := challengeAEAD(key)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(sealedChallenge{Answer: answer, Expires: expires.Unix()})
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, nil)), nil
}

func openChallenge(key []byte, id string, now time.Time) ([]byte, time.Time, error) {
	aead, err := challengeAEAD(key)
	if err != nil {
		return nil, time.Time{}, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) < aead.NonceSize() {
		return nil, time.Time{}, captcha.ErrInvalid
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return nil, time.Time{}, captcha.ErrInvalid
	}
	var sealed sealedChallenge
	if err := json.Unmarshal(plain, &sealed); err != nil {
		return nil, time.Time{}, captcha.ErrInvalid
	}
	expires := time.Unix(sealed.Expires, 0)
	if !expires.After(now) {
		return nil, time.Time{}, fmt.Errorf("%w: expired", captcha.ErrInvalid)
	}
	return sealed.Answer, expires, nil
}

func challengeAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("captcha seal key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// challengeUseKey is the sys_captcha_used key of a challenge ID.
func challengeUseKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
