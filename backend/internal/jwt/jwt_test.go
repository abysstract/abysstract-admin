package jwt

import (
	"testing"
	"time"
)

// Feature: open-source-readiness, Property 4: JWT 签发-校验往返
func TestJWTGenerateAndParse(t *testing.T) {
	secret := "test-secret-key-32bytes!!!"
	expiresIn := 1 * time.Hour
	mgr := NewManager(secret, expiresIn)

	userID := int64(42)
	username := "alice"

	token, expiresInSec, err := mgr.Generate(userID, username, 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if expiresInSec != int64(expiresIn.Seconds()) {
		t.Errorf("expiresIn = %d, want %d", expiresInSec, int64(expiresIn.Seconds()))
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %d, want %d", claims.UserID, userID)
	}
	if claims.Username != username {
		t.Errorf("Username = %s, want %s", claims.Username, username)
	}
}

// Feature: open-source-readiness, Property 4: 不同密钥校验失败
func TestJWTParseWithDifferentSecret(t *testing.T) {
	mgrA := NewManager("secret-aaaa", 1*time.Hour)
	mgrB := NewManager("secret-bbbb", 1*time.Hour)

	token, _, err := mgrA.Generate(1, "test", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = mgrB.Parse(token)
	if err == nil {
		t.Fatal("expected error when parsing with different secret, got nil")
	}
}

// Feature: open-source-readiness, Property 4: 过期 token 校验失败
func TestJWTExpiredToken(t *testing.T) {
	mgr := NewManager("secret", -1*time.Second) // 立即过期

	token, _, err := mgr.Generate(1, "test", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = mgr.Parse(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}
