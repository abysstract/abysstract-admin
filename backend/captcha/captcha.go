// Package captcha defines how the login asks for and checks a human
// verification after repeated failures.
//
// A Verifier checks what the client submits with a login. A Challenger also
// issues the challenge the client renders (GetCaptcha). Implementations are
// selected by name through Register and Open, like storage drivers:
//
//   - captcha/slidecaptcha registers "slide", a self-hosted slider puzzle.
//   - captcha/siteverify registers "turnstile" and "hcaptcha", which check the
//     token of Cloudflare Turnstile or hCaptcha with the provider.
//
// Applications with another service implement Verifier (and Challenger when
// the server issues the challenge) and pass it to adminapp.Options.Captcha.
package captcha

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// ErrInvalid is returned by Verify when the answer is wrong, expired or
// already used. Any other error means the check could not be made, and the
// login is refused as unavailable rather than let through.
var ErrInvalid = errors.New("captcha: invalid answer")

// Submission is what Verify checks.
type Submission struct {
	// Token is what the client submitted: the provider token, or the answer
	// to a Challenge in the format the Challenger defines.
	Token string
	// Answer is Challenge.Answer of the challenge the client answered. The
	// application keeps it tamper-proof and single-use between Challenge and
	// Verify. It is nil for verifiers whose challenges carry no answer.
	Answer []byte
	// RemoteIP is the client address, forwarded to providers that use it.
	RemoteIP string
}

// Verifier checks a submission. It returns nil when it passes, ErrInvalid
// (possibly wrapped) when it does not, and another error when the check
// itself failed.
type Verifier interface {
	Verify(ctx context.Context, sub Submission) error
}

// Challenge is one challenge for the client.
type Challenge struct {
	// Kind names the client component that renders Payload, such as "slide"
	// or "turnstile".
	Kind string
	// Payload is JSON for the client component.
	Payload string
	// Answer stays on the server and is passed back in Submission.Answer.
	// Leave it nil when the provider checks the answer itself.
	Answer []byte
}

// Challenger is a Verifier that tells the client what to render.
type Challenger interface {
	Verifier
	Challenge(ctx context.Context) (Challenge, error)
}

// Config selects and configures an implementation for Open. Fields that an
// implementation does not use are ignored.
type Config struct {
	// Provider is the registered name, such as "slide" or "turnstile".
	Provider string
	// Secret is the server-side key of a third-party provider.
	Secret string
	// SiteKey is the public key of a third-party provider, returned to the
	// client in the challenge payload.
	SiteKey string
	// HTTPClient calls third-party providers; nil uses a client with a
	// 10 second timeout.
	HTTPClient *http.Client
}

// Factory builds a Verifier from configuration.
type Factory func(cfg Config) (Verifier, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes an implementation available to Open. It panics when name
// is empty, factory is nil or name is already registered; implementations
// call it from init.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if name == "" || factory == nil {
		panic("captcha: Register needs a name and a factory")
	}
	if _, dup := registry[name]; dup {
		panic("captcha: provider " + name + " registered twice")
	}
	registry[name] = factory
}

// Registered reports whether an implementation named name is available.
func Registered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// Providers returns the registered names in sorted order.
func Providers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Open builds the implementation selected by cfg.Provider.
func Open(cfg Config) (Verifier, error) {
	registryMu.RLock()
	factory := registry[cfg.Provider]
	registryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("captcha: provider %q is not registered (available: %v); import its package, for example captcha/slidecaptcha", cfg.Provider, Providers())
	}
	return factory(cfg)
}
