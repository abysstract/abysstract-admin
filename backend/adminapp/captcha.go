package adminapp

import (
	"fmt"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/captcha"

	// Turnstile and hCaptcha need no extra dependencies and are always
	// available; the self-hosted slider is enabled by importing
	// captcha/slidecaptcha.
	_ "github.com/abysstract/abysstract-admin/backend/captcha/siteverify"
)

// CaptchaConfig maps the application configuration to the captcha
// configuration used by OpenCaptcha.
func CaptchaConfig(cfg adminconfig.Config) captcha.Config {
	return captcha.Config{Provider: cfg.CaptchaProvider, Secret: cfg.CaptchaSecret, SiteKey: cfg.CaptchaSiteKey}
}

// OpenCaptcha opens the provider named by ADMIN_CAPTCHA_PROVIDER. It returns
// nil without error when no provider is configured.
func OpenCaptcha(cfg adminconfig.Config) (captcha.Verifier, error) {
	if cfg.CaptchaProvider == "" {
		return nil, nil
	}
	if !captcha.Registered(cfg.CaptchaProvider) {
		return nil, fmt.Errorf("adminapp: captcha provider %q is not registered (available: %v); import its package, for example captcha/slidecaptcha, or pass Options.Captcha",
			cfg.CaptchaProvider, captcha.Providers())
	}
	v, err := captcha.Open(CaptchaConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("adminapp: %w", err)
	}
	return v, nil
}
