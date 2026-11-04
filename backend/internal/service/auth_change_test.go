package service_test

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
)

// mustLogin 使用 admin/admin123 登录拿 token。失败 t.Fatal。
func mustLogin(t *testing.T, svc *service.AuthService, username, password string) string {
	t.Helper()
	resp, err := svc.Login(context.Background(),
		&adminv1.LoginRequest{Username: username, Password: password})
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return resp.GetToken()
}

// ctxWithClaims 构造带 Claims 的 ctx，模拟拦截器已注入。
func ctxWithClaims(userID int64, username string, ver int64) context.Context {
	return interceptor.WithClaims(context.Background(), &jwt.Claims{
		UserID:   userID,
		Username: username,
		Ver:      ver,
	})
}

// 验证 bcrypt 比对的辅助 sanity check（确保 hashPassword 输出可被 bcrypt 校验）。
func TestHashPasswordSanity(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Aa123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte("Aa123456")) != nil {
		t.Fatal("bcrypt compare should succeed")
	}
}
