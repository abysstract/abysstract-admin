package adminconfig

import (
	"os"
	"strings"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

// 登录验证码与按 IP 计数的默认值。
const (
	defaultLoginCaptchaAfter      = 3
	defaultLoginIPCaptchaAfter    = 10
	defaultLoginIPMaxFailures     = 30
	defaultLoginIPWindowSeconds   = 900
	defaultLoginMaxFailuresSolved = 20 // 启用验证码后的默认锁定阈值
)

func (c *Config) loadLoginGuard() {
	c.LoginCaptchaAfter = c.getInt("ADMIN_LOGIN_CAPTCHA_AFTER", defaultLoginCaptchaAfter)
	c.LoginIPCaptchaAfter = c.getInt("ADMIN_LOGIN_IP_CAPTCHA_AFTER", defaultLoginIPCaptchaAfter)
	c.LoginIPMaxFailures = c.getInt("ADMIN_LOGIN_IP_MAX_FAILURES", defaultLoginIPMaxFailures)
	c.LoginIPWindowSeconds = c.getInt("ADMIN_LOGIN_IP_WINDOW_SECONDS", defaultLoginIPWindowSeconds)
	c.CaptchaProvider = strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_CAPTCHA_PROVIDER")))
	c.CaptchaSecret = os.Getenv("ADMIN_CAPTCHA_SECRET")
	c.CaptchaSiteKey = strings.TrimSpace(os.Getenv("ADMIN_CAPTCHA_SITE_KEY"))
	if raw := os.Getenv("ADMIN_TRUSTED_PROXIES"); raw != "" {
		proxies, err := accesslog.ParseProxies(raw)
		if err != nil {
			c.parseErrs = append(c.parseErrs, "ADMIN_TRUSTED_PROXIES "+err.Error())
		}
		c.TrustedProxies = proxies
	}
}

func (c Config) loginGuardProblems() []string {
	var problems []string
	check := func(key string, v int) {
		if v < 1 {
			problems = append(problems, key+" 必须 ≥ 1")
		}
	}
	check("ADMIN_LOGIN_CAPTCHA_AFTER", c.LoginCaptchaAfterOrDefault())
	check("ADMIN_LOGIN_IP_CAPTCHA_AFTER", c.LoginIPCaptchaAfterOrDefault())
	check("ADMIN_LOGIN_IP_MAX_FAILURES", c.LoginIPMaxFailuresOrDefault())
	check("ADMIN_LOGIN_IP_WINDOW_SECONDS", c.LoginIPWindowSecondsOrDefault())
	return problems
}

// ProxyPolicy returns how client IPs are read from X-Forwarded-For
// (ADMIN_TRUST_PROXY, ADMIN_TRUSTED_PROXIES).
func (c Config) ProxyPolicy() accesslog.ProxyPolicy {
	return accesslog.ProxyPolicy{Trust: c.TrustProxy, Proxies: c.TrustedProxies}
}

// LoginMaxFailuresFor returns the account lockout threshold. An explicit
// ADMIN_LOGIN_MAX_FAILURES wins; otherwise it is 5 without a captcha and 20
// with one, where the captcha already slows down guessing and a low
// threshold would let anyone lock an account out with a few solved captchas.
func (c Config) LoginMaxFailuresFor(captcha bool) int {
	if c.LoginMaxFailures > 0 {
		return c.LoginMaxFailures
	}
	if captcha {
		return defaultLoginMaxFailuresSolved
	}
	return defaultLoginMaxFailures
}

// LoginCaptchaAfterOrDefault returns LoginCaptchaAfter, or 3 when unset.
func (c Config) LoginCaptchaAfterOrDefault() int {
	return positiveOr(c.LoginCaptchaAfter, defaultLoginCaptchaAfter)
}

// LoginIPCaptchaAfterOrDefault returns LoginIPCaptchaAfter, or 10 when unset.
func (c Config) LoginIPCaptchaAfterOrDefault() int {
	return positiveOr(c.LoginIPCaptchaAfter, defaultLoginIPCaptchaAfter)
}

// LoginIPMaxFailuresOrDefault returns LoginIPMaxFailures, or 30 when unset.
func (c Config) LoginIPMaxFailuresOrDefault() int {
	return positiveOr(c.LoginIPMaxFailures, defaultLoginIPMaxFailures)
}

// LoginIPWindowSecondsOrDefault returns LoginIPWindowSeconds, or 900 when unset.
func (c Config) LoginIPWindowSecondsOrDefault() int {
	return positiveOr(c.LoginIPWindowSeconds, defaultLoginIPWindowSeconds)
}

// positiveOr keeps zero-valued Config literals usable: 0 selects the
// default, negative values stay invalid for Validate.
func positiveOr(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
