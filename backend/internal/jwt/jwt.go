// Package jwt 提供 JWT 的签发与校验。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义负载：携带用户 ID、登录名与 token 版本号。
// Ver 用于服务端会话失效：改密/登出后 user.token_version 递增，
// 拦截器比对 claims.Ver == user.token_version，不等则拒绝。
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Ver      int64  `json:"ver,omitempty"`
	jwt.RegisteredClaims
}

// Manager 负责 token 的签发与解析。
type Manager struct {
	secret    []byte
	expiresIn time.Duration
}

func NewManager(secret string, expiresIn time.Duration) *Manager {
	return &Manager{secret: []byte(secret), expiresIn: expiresIn}
}

// Generate 为指定用户签发 token，返回 token 字符串与有效期（秒）。
// ver 为该用户当前的 token_version，写入 claims 供拦截器比对。
func (m *Manager) Generate(userID int64, username string, ver int64) (string, int64, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Ver:      ver,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiresIn)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, int64(m.expiresIn.Seconds()), nil
}

// Parse 校验并解析 token。
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
