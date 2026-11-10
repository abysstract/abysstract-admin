package accesslog

import (
	"context"
	"maps"
	"sync"
)

// record is the mutable entry of the request in flight. Middleware or
// Interceptor creates it; handlers and auth code enrich it through the
// context helpers.
type record struct {
	mu    sync.Mutex
	entry Entry
	skip  bool
	rpc   bool
}

type recordKey struct{}

func withRecord(ctx context.Context, r *record) context.Context {
	return context.WithValue(ctx, recordKey{}, r)
}

func recordFrom(ctx context.Context) *record {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(recordKey{}).(*record)
	return r
}

// Active reports whether ctx belongs to a request whose entry is being
// captured.
func Active(ctx context.Context) bool { return recordFrom(ctx) != nil }

// SetPrincipal records the authenticated caller of the current request.
// Authentication code calls it after verifying credentials. It reports false
// when ctx carries no entry.
func SetPrincipal(ctx context.Context, p Principal) bool {
	r := recordFrom(ctx)
	if r == nil {
		return false
	}
	r.mu.Lock()
	r.entry.Principal = p
	r.mu.Unlock()
	return true
}

// PrincipalFrom returns the principal recorded for the current request.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	r := recordFrom(ctx)
	if r == nil {
		return Principal{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.entry.Principal, !r.entry.Principal.IsZero()
}

// SetAttr sets one business field on the current entry, for example
// SetAttr(ctx, "items_ok", 42). value must be JSON-encodable. It reports
// false when ctx carries no entry.
func SetAttr(ctx context.Context, key string, value any) bool {
	r := recordFrom(ctx)
	if r == nil || key == "" {
		return false
	}
	r.mu.Lock()
	if r.entry.Attrs == nil {
		r.entry.Attrs = map[string]any{}
	}
	r.entry.Attrs[key] = value
	r.mu.Unlock()
	return true
}

// AddAttrs merges business fields into the current entry. Later values
// replace earlier ones with the same key. It reports false when ctx carries
// no entry.
func AddAttrs(ctx context.Context, attrs map[string]any) bool {
	r := recordFrom(ctx)
	if r == nil {
		return false
	}
	r.mu.Lock()
	if r.entry.Attrs == nil {
		r.entry.Attrs = make(map[string]any, len(attrs))
	}
	for k, v := range attrs {
		if k != "" {
			r.entry.Attrs[k] = v
		}
	}
	r.mu.Unlock()
	return true
}

// SetError records an error message for a request that does not return a
// Connect error, such as a plain HTTP handler answering 4xx or 5xx.
func SetError(ctx context.Context, msg string) bool {
	r := recordFrom(ctx)
	if r == nil {
		return false
	}
	r.mu.Lock()
	r.entry.Error = truncate(msg, MaxErrorLength)
	r.mu.Unlock()
	return true
}

// Skip drops the entry of the current request, for example for a noisy
// polling endpoint.
func Skip(ctx context.Context) bool {
	r := recordFrom(ctx)
	if r == nil {
		return false
	}
	r.mu.Lock()
	r.skip = true
	r.mu.Unlock()
	return true
}

// snapshot returns a copy of the entry that later context updates cannot
// change, or false when the entry was skipped.
func (r *record) snapshot() (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.skip {
		return Entry{}, false
	}
	e := r.entry
	if e.Attrs != nil {
		e.Attrs = maps.Clone(e.Attrs)
	}
	return e, true
}

// truncate shortens s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && cut < len(s) && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}
