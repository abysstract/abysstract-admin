// Package siteverify checks Cloudflare Turnstile and hCaptcha tokens with
// the provider's siteverify endpoint. Importing it registers the captcha
// providers "turnstile" and "hcaptcha".
//
// The client renders the provider's widget with Config.SiteKey (returned by
// GetCaptcha) and submits the token it produces; the token is single-use and
// checked by the provider, so no answer is kept on the server.
package siteverify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abysstract/abysstract-admin/backend/captcha"
)

// Siteverify endpoints of the built-in providers.
const (
	TurnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	HCaptchaEndpoint  = "https://api.hcaptcha.com/siteverify"
)

func init() {
	captcha.Register("turnstile", func(cfg captcha.Config) (captcha.Verifier, error) {
		return New("turnstile", TurnstileEndpoint, cfg)
	})
	captcha.Register("hcaptcha", func(cfg captcha.Config) (captcha.Verifier, error) {
		return New("hcaptcha", HCaptchaEndpoint, cfg)
	})
}

// Verifier posts tokens to a siteverify endpoint. It implements
// captcha.Challenger so GetCaptcha can tell the client which widget and site
// key to use.
type Verifier struct {
	kind     string
	endpoint string
	secret   string
	siteKey  string
	client   *http.Client
}

// New returns a Verifier for any provider that speaks the siteverify
// protocol: a form POST of secret, response and remoteip answered with
// {"success": bool, "error-codes": [...]}. kind is passed to the client as
// Challenge.Kind.
func New(kind, endpoint string, cfg captcha.Config) (*Verifier, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("captcha: %s needs a secret (ADMIN_CAPTCHA_SECRET)", kind)
	}
	if cfg.SiteKey == "" {
		return nil, fmt.Errorf("captcha: %s needs a site key (ADMIN_CAPTCHA_SITE_KEY)", kind)
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Verifier{kind: kind, endpoint: endpoint, secret: cfg.Secret, siteKey: cfg.SiteKey, client: client}, nil
}

// Challenge tells the client to render the provider widget.
func (v *Verifier) Challenge(context.Context) (captcha.Challenge, error) {
	payload, err := json.Marshal(map[string]string{"siteKey": v.siteKey})
	if err != nil {
		return captcha.Challenge{}, err
	}
	return captcha.Challenge{Kind: v.kind, Payload: string(payload)}, nil
}

type response struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify asks the provider whether the token is valid.
func (v *Verifier) Verify(ctx context.Context, sub captcha.Submission) error {
	if strings.TrimSpace(sub.Token) == "" {
		return captcha.ErrInvalid
	}
	form := url.Values{"secret": {v.secret}, "response": {sub.Token}}
	if sub.RemoteIP != "" {
		form.Set("remoteip", sub.RemoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("captcha: %s siteverify: %w", v.kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("captcha: %s siteverify: HTTP %d", v.kind, resp.StatusCode)
	}
	var out response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return fmt.Errorf("captcha: %s siteverify: %w", v.kind, err)
	}
	if out.Success {
		return nil
	}
	for _, code := range out.ErrorCodes {
		// A wrong secret is a deployment error, not a wrong answer: report it
		// so the login fails as unavailable and the logs show why.
		if strings.Contains(code, "secret") || code == "internal-error" {
			return fmt.Errorf("captcha: %s siteverify: %s", v.kind, strings.Join(out.ErrorCodes, ","))
		}
	}
	return errors.Join(captcha.ErrInvalid, fmt.Errorf("%s: %s", v.kind, strings.Join(out.ErrorCodes, ",")))
}
