package accesslog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// Request ID bounds accepted from clients by CaptureOptions.AssignRequestID.
const (
	MinRequestIDLength = 8
	MaxRequestIDLength = 128
)

type requestIDKey struct{}

// WithRequestID returns ctx carrying the request ID id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID assigned by Middleware or
// Interceptor with CaptureOptions.AssignRequestID, or set by WithRequestID.
// It returns "" when there is none.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// ValidRequestID reports whether a client supplied request ID is safe to
// keep: MinRequestIDLength to MaxRequestIDLength characters of ASCII
// letters, digits and "-_.:+/=@". Other values are replaced, so clients
// cannot inject control characters or oversized values into logs.
func ValidRequestID(id string) bool {
	if len(id) < MinRequestIDLength || len(id) > MaxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("-_.:+/=@", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// NewRequestID returns a random request ID of 96 bits as 24 hex characters.
func NewRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; keep requests
		// distinguishable if it ever does.
		return hex.EncodeToString([]byte(time.Now().UTC().Format("060102150405.000")))[:24]
	}
	return hex.EncodeToString(b[:])
}

// resolveRequestID keeps a valid client request ID and generates one
// otherwise.
func resolveRequestID(header string) string {
	if ValidRequestID(header) {
		return header
	}
	return NewRequestID()
}

// DefaultSkip reports whether a request is CORS preflight (OPTIONS) or one
// of the probe and metrics paths /healthz, /readyz, /metrics and /version.
// The admin application uses it; separate servers can pass it as
// CaptureOptions.Skip or extend it with SkipPaths.
func DefaultSkip(r *http.Request) bool {
	if r.Method == http.MethodOptions {
		return true
	}
	switch r.URL.Path {
	case "/healthz", "/readyz", "/metrics", "/version":
		return true
	}
	return false
}

// SkipPaths returns a CaptureOptions.Skip that skips what DefaultSkip skips
// and the given paths. A path ending in "/" matches every path below it, such
// as "/static/"; other paths match exactly.
func SkipPaths(paths ...string) func(*http.Request) bool {
	exact := map[string]bool{}
	var prefixes []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/") {
			prefixes = append(prefixes, p)
		} else {
			exact[p] = true
		}
	}
	return func(r *http.Request) bool {
		if DefaultSkip(r) || exact[r.URL.Path] {
			return true
		}
		for _, p := range prefixes {
			if strings.HasPrefix(r.URL.Path, p) {
				return true
			}
		}
		return false
	}
}
