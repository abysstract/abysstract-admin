package service

import (
	"strings"
	"testing"
	"testing/quick"
	"unicode"
	"unicode/utf8"
)

// Property 8: 密码复杂度策略（需求 6.3）
//
// 不变量：
//  1. 长度 < 8 → errPasswordTooShort。
//  2. 长度 > 64 → errPasswordTooLong。
//  3. 长度 8–64 且大写/小写/数字三类中至少两类 → nil。
//  4. 长度 8–64 但不足两类 → errPasswordWeak。
//
// 使用 testing/quick 对任意 Unicode 字符串验证：策略返回值必须与按字符分类预测一致。
func TestValidatePasswordPolicy_Property(t *testing.T) {
	predict := func(pwd string) bool {
		n := utf8.RuneCountInString(pwd)
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

		err := validatePasswordPolicy(pwd)
		switch {
		case n < 8:
			return err == errPasswordTooShort
		case n > 64:
			return err == errPasswordTooLong
		case classes < 2:
			return err == errPasswordWeak
		default:
			return err == nil
		}
	}

	if err := quick.Check(predict, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatalf("property check failed: %v", err)
	}
}

// TestValidatePasswordPolicy_Boundaries 显式覆盖关键边界。
func TestValidatePasswordPolicy_Boundaries(t *testing.T) {
	cases := []struct {
		name string
		pwd  string
		want error
	}{
		{"empty", "", errPasswordTooShort},
		{"7_chars_single_class", "aaaaaaa", errPasswordTooShort},
		{"8_chars_single_class", "aaaaaaaa", errPasswordWeak},
		{"8_chars_two_classes", "Aaaaaaaa", nil},
		{"8_chars_two_classes_lower_digit", "aaaaaaa1", nil},
		{"64_chars_two_classes", strings.Repeat("aA", 32), nil},
		{"65_chars_two_classes", strings.Repeat("aA", 32) + "a", errPasswordTooLong},
		{"digit_only", "12345678", errPasswordWeak},
		{"upper_only", "ABCDEFGH", errPasswordWeak},
		{"chinese_only", strings.Repeat("中", 8), errPasswordWeak}, // 中文字符不属任一类
		{"chinese_with_digit_and_lower", "中中中中中中a1", nil},         // 小写+数字两类
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validatePasswordPolicy(c.pwd)
			if err != c.want {
				t.Errorf("pwd=%q err=%v, want %v", c.pwd, err, c.want)
			}
		})
	}
}
