// Package googleid verifies the ID tokens issued by Google Identity Services
// ("Sign in with Google"). The browser obtains the token from Google and the
// server only needs the application's OAuth client ID: tokens are checked
// against Google's published signing keys, so no client secret is involved.
package googleid

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CertsURL is where Google publishes the keys that sign ID tokens.
const CertsURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	defaultKeyTTL = time.Hour
	// minRefresh limits how often an unknown key ID triggers a refetch, so a
	// stream of forged tokens cannot turn into a stream of requests to Google.
	minRefresh = time.Minute
	maxKeys    = 1 << 16 // bytes read from the certs response
)

// ErrInvalidToken is returned when the token is malformed, expired, issued
// for another client, or not signed by Google. ErrUnavailable is returned when
// Google's keys cannot be fetched; it says nothing about the token.
var (
	ErrInvalidToken = errors.New("googleid: invalid ID token")
	ErrUnavailable  = errors.New("googleid: cannot fetch Google signing keys")
)

// Claims are the facts about the signed-in Google account.
type Claims struct {
	Subject       string // stable account ID ("sub"); never reused
	Email         string
	EmailVerified bool
	HostedDomain  string // Google Workspace domain ("hd"); empty for other accounts
	Name          string
}

// Config configures a Verifier.
type Config struct {
	// ClientID is the OAuth client ID the token must have been issued for.
	ClientID string
	// Domains, when non-empty, restricts sign-in to these domains. A
	// Workspace account is matched by its hosted domain; a consumer account
	// only by "gmail.com". An address on another domain that is not a
	// Workspace account is refused even if its domain is listed.
	Domains []string
	// HTTPClient fetches the signing keys; http.DefaultClient with a short
	// timeout when nil.
	HTTPClient *http.Client
	// CertsURL overrides the keys endpoint (tests).
	CertsURL string
	// Now overrides the clock (tests).
	Now func() time.Time
}

// Verifier checks Google ID tokens. It is safe for concurrent use.
type Verifier struct {
	clientID string
	domains  map[string]bool
	client   *http.Client
	certs    string
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expires   time.Time
	fetchedAt time.Time
}

// New returns a Verifier for cfg.
func New(cfg Config) (*Verifier, error) {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("googleid: client ID is required")
	}
	v := &Verifier{
		clientID: strings.TrimSpace(cfg.ClientID),
		client:   cfg.HTTPClient,
		certs:    cfg.CertsURL,
		now:      cfg.Now,
	}
	if v.client == nil {
		v.client = &http.Client{Timeout: 10 * time.Second}
	}
	if v.certs == "" {
		v.certs = CertsURL
	}
	if v.now == nil {
		v.now = time.Now
	}
	if len(cfg.Domains) > 0 {
		v.domains = make(map[string]bool, len(cfg.Domains))
		for _, d := range cfg.Domains {
			if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
				v.domains[d] = true
			}
		}
	}
	return v, nil
}

// ClientID returns the OAuth client ID tokens must be issued for.
func (v *Verifier) ClientID() string { return v.clientID }

type tokenClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // bool, or "true" in some tokens
	HD            string `json:"hd"`
	Name          string `json:"name"`
}

// Verify checks the token's signature, issuer, audience and expiry, that the
// email is verified, and the optional domain restriction.
func (v *Verifier) Verify(ctx context.Context, idToken string) (Claims, error) {
	var tc tokenClaims
	_, err := jwt.ParseWithClaims(idToken, &tc, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(v.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(v.now),
		jwt.WithLeeway(time.Minute),
	)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return Claims{}, err
		}
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if iss := tc.Issuer; iss != "https://accounts.google.com" && iss != "accounts.google.com" {
		return Claims{}, fmt.Errorf("%w: issuer %q", ErrInvalidToken, iss)
	}
	if tc.Subject == "" || tc.Email == "" {
		return Claims{}, fmt.Errorf("%w: no subject or email", ErrInvalidToken)
	}
	claims := Claims{
		Subject:       tc.Subject,
		Email:         tc.Email,
		EmailVerified: truthy(tc.EmailVerified),
		HostedDomain:  strings.ToLower(tc.HD),
		Name:          tc.Name,
	}
	if !claims.EmailVerified {
		return Claims{}, fmt.Errorf("%w: email is not verified", ErrInvalidToken)
	}
	if v.domains != nil && !v.domains[accountDomain(claims)] {
		return Claims{}, fmt.Errorf("%w: domain is not allowed", ErrInvalidToken)
	}
	return claims, nil
}

// accountDomain is the domain used for the allow-list: the Workspace domain,
// or gmail.com for a consumer Gmail address, and nothing otherwise.
func accountDomain(c Claims) string {
	if c.HostedDomain != "" {
		return c.HostedDomain
	}
	if strings.HasSuffix(strings.ToLower(c.Email), "@gmail.com") {
		return "gmail.com"
	}
	return ""
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	}
	return false
}

// key returns the public key with the given ID, refreshing the cached set
// when it has expired or the ID is unknown.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now()
	if v.keys != nil && now.Before(v.expires) {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
	}
	if v.keys != nil && now.Sub(v.fetchedAt) < minRefresh {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	if err := v.refreshLocked(ctx, now); err != nil {
		return nil, err
	}
	k, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	return k, nil
}

type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *Verifier) refreshLocked(ctx context.Context, now time.Time) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certs, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	var set jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxKeys)).Decode(&set); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(n) == 0 || len(e) == 0 || len(e) > 4 {
			continue
		}
		exp := 0
		for _, b := range e {
			exp = exp<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}
	}
	if len(keys) == 0 {
		return fmt.Errorf("%w: no usable keys", ErrUnavailable)
	}
	v.keys, v.fetchedAt, v.expires = keys, now, now.Add(keyTTL(resp.Header.Get("Cache-Control")))
	return nil
}

var maxAge = regexp.MustCompile(`max-age=(\d+)`)

func keyTTL(cacheControl string) time.Duration {
	if m := maxAge.FindStringSubmatch(cacheControl); m != nil {
		if s, err := strconv.Atoi(m[1]); err == nil && s > 0 {
			return time.Duration(s) * time.Second
		}
	}
	return defaultKeyTTL
}
