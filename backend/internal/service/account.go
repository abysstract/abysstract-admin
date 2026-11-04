package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// 密码复杂度策略常量（需求 6.3）。
const (
	passwordMinLen = 8
	passwordMaxLen = 64
)

var (
	errPasswordTooShort = errors.New("密码长度至少 8 位")
	errPasswordTooLong  = errors.New("密码长度不得超过 64 位")
	errPasswordWeak     = errors.New("密码必须包含大写字母、小写字母、数字三类中的至少两类")
)

// validatePasswordPolicy 校验密码复杂度（需求 6.3）。
// 规则：长度 8–64（按字符计数），且大写/小写/数字三类中至少匹配两类。
// 纯函数：无外部依赖，便于属性测试覆盖。
func validatePasswordPolicy(pwd string) error {
	n := utf8.RuneCountInString(pwd)
	if n < passwordMinLen {
		return errPasswordTooShort
	}
	if n > passwordMaxLen {
		return errPasswordTooLong
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range pwd {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	classes := 0
	if hasUpper {
		classes++
	}
	if hasLower {
		classes++
	}
	if hasDigit {
		classes++
	}
	if classes < 2 {
		return errPasswordWeak
	}
	return nil
}

// hashPassword 计算 bcrypt 哈希，包装 bcrypt 错误。
func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// verifyPassword 包装 bcrypt 比对；不返回具体原因避免泄漏时机。
func verifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// hashRefreshToken 返回 SHA-256 hex，用于 sys_refresh_token 存储（避免明文）。
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
