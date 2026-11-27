package googleid

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testClient = "client-123.apps.googleusercontent.com"

type fixture struct {
	key    *rsa.PrivateKey
	server *httptest.Server
	hits   atomic.Int32
	now    time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{key: key, now: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) verifier(t *testing.T, domains ...string) *Verifier {
	t.Helper()
	v, err := New(Config{ClientID: testClient, Domains: domains, CertsURL: f.server.URL, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// token signs a token that is valid unless mutate changes it.
func (f *fixture) token(t *testing.T, mutate func(claims jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClient, "sub": "1001",
		"email": "Alice@Corp.com", "email_verified": true, "hd": "corp.com", "name": "Alice",
		"iat": f.now.Unix(), "exp": f.now.Add(time.Hour).Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	f := newFixture(t)
	got, err := f.verifier(t).Verify(context.Background(), f.token(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "1001" || got.Email != "Alice@Corp.com" || got.HostedDomain != "corp.com" || !got.EmailVerified {
		t.Fatalf("claims = %+v", got)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := newFixture(t)
	v := f.verifier(t)
	cases := map[string]func(c jwt.MapClaims){
		"other client":     func(c jwt.MapClaims) { c["aud"] = "someone-else" },
		"expired":          func(c jwt.MapClaims) { c["exp"] = f.now.Add(-time.Hour).Unix() },
		"no expiry":        func(c jwt.MapClaims) { delete(c, "exp") },
		"other issuer":     func(c jwt.MapClaims) { c["iss"] = "https://evil.example" },
		"email unverified": func(c jwt.MapClaims) { c["email_verified"] = false },
		"no email":         func(c jwt.MapClaims) { delete(c, "email") },
		"no subject":       func(c jwt.MapClaims) { delete(c, "sub") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), f.token(t, mutate))
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyAcceptsBareIssuerAndStringVerified(t *testing.T) {
	f := newFixture(t)
	_, err := f.verifier(t).Verify(context.Background(), f.token(t, func(c jwt.MapClaims) {
		c["iss"] = "accounts.google.com"
		c["email_verified"] = "true"
	}))
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsWrongSignatureAndAlgorithm(t *testing.T) {
	f := newFixture(t)
	v := f.verifier(t)

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	forged := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClient, "sub": "1", "email": "a@b.c",
		"email_verified": true, "exp": f.now.Add(time.Hour).Unix(),
	})
	forged.Header["kid"] = "k1"
	s, _ := forged.SignedString(other)
	if _, err := v.Verify(context.Background(), s); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("forged signature: err = %v", err)
	}

	// 算法混淆：用公钥字节当 HMAC 密钥签名。
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClient, "sub": "1", "email": "a@b.c",
		"email_verified": true, "exp": f.now.Add(time.Hour).Unix(),
	})
	hs.Header["kid"] = "k1"
	s, _ = hs.SignedString(f.key.N.Bytes())
	if _, err := v.Verify(context.Background(), s); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS256 token: err = %v", err)
	}

	// alg=none
	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"aud": testClient, "exp": f.now.Add(time.Hour).Unix()})
	none.Header["kid"] = "k1"
	s, _ = none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := v.Verify(context.Background(), s); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("alg none: err = %v", err)
	}
}

func TestVerifyDomains(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name    string
		domains []string
		mutate  func(c jwt.MapClaims)
		ok      bool
	}{
		{"workspace match", []string{"corp.com"}, nil, true},
		{"workspace case-insensitive list", []string{"CORP.com"}, nil, true},
		{"workspace other domain", []string{"other.com"}, nil, false},
		{"gmail listed", []string{"gmail.com"}, func(c jwt.MapClaims) { delete(c, "hd"); c["email"] = "a@gmail.com" }, true},
		{"gmail not listed", []string{"corp.com"}, func(c jwt.MapClaims) { delete(c, "hd"); c["email"] = "a@gmail.com" }, false},
		// 用公司域名邮箱注册的普通 Google 账号没有 hd，不能冒充 Workspace 成员。
		{"corp email without hd", []string{"corp.com"}, func(c jwt.MapClaims) { delete(c, "hd") }, false},
		{"no restriction", nil, func(c jwt.MapClaims) { delete(c, "hd"); c["email"] = "a@anywhere.org" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.verifier(t, tc.domains...).Verify(context.Background(), f.token(t, tc.mutate))
			if tc.ok && err != nil {
				t.Fatalf("err = %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestKeysAreCachedAndRefetchIsThrottled(t *testing.T) {
	f := newFixture(t)
	v := f.verifier(t)
	for range 3 {
		if _, err := v.Verify(context.Background(), f.token(t, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.hits.Load(); n != 1 {
		t.Fatalf("keys fetched %d times, want 1", n)
	}

	// 未知 kid 在冷却期内不会再次请求 Google。
	bad := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"aud": testClient, "exp": f.now.Add(time.Hour).Unix()})
	bad.Header["kid"] = "unknown"
	s, _ := bad.SignedString(f.key)
	for range 3 {
		if _, err := v.Verify(context.Background(), s); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v", err)
		}
	}
	if n := f.hits.Load(); n != 1 {
		t.Fatalf("unknown kid triggered a refetch: %d fetches", n)
	}
}

func TestVerifyReportsUnavailableKeys(t *testing.T) {
	f := newFixture(t)
	f.server.Close() // Google 不可达
	_, err := f.verifier(t).Verify(context.Background(), f.token(t, nil))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if errors.Is(err, ErrInvalidToken) {
		t.Fatal("unreachable Google must not be reported as an invalid token")
	}
}

func TestNewRequiresClientID(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("want error")
	}
}
