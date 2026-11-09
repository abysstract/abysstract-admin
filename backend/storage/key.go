package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"
)

// MaxKeyLength is the longest key accepted by ValidateKey, in bytes. It
// matches the S3 object key limit.
const MaxKeyLength = 1024

// ErrInvalidKey reports a key rejected by ValidateKey.
var ErrInvalidKey = errors.New("storage: invalid key")

// ValidateKey checks that key is a relative slash-separated path that every
// driver can store safely. It rejects empty keys, absolute paths, backslashes,
// control characters, empty segments ("a//b", trailing "/") and segments that
// start with "." (which covers "." and "..").
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if len(key) > MaxKeyLength {
		return fmt.Errorf("%w: longer than %d bytes", ErrInvalidKey, MaxKeyLength)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: %q is absolute", ErrInvalidKey, key)
	}
	for _, r := range key {
		if r == '\\' || unicode.IsControl(r) || r == unicode.ReplacementChar {
			return fmt.Errorf("%w: %q contains a forbidden character", ErrInvalidKey, key)
		}
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" {
			return fmt.Errorf("%w: %q has an empty segment", ErrInvalidKey, key)
		}
		if strings.HasPrefix(seg, ".") {
			return fmt.Errorf("%w: %q has a segment starting with \".\"", ErrInvalidKey, key)
		}
	}
	return nil
}

// JoinKey joins segments with "/" and validates the result. Each part may
// itself contain "/"; empty parts are an error rather than being skipped.
func JoinKey(parts ...string) (string, error) {
	key := strings.Join(parts, "/")
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return key, nil
}

// RandomKey returns prefix + "/" + 32 random hex characters + the lowercased
// extension of filename. An empty prefix yields a single-segment key.
func RandomKey(prefix, filename string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("storage: random key: %w", err)
	}
	name := hex.EncodeToString(b[:]) + Ext(filename)
	if prefix == "" {
		return JoinKey(name)
	}
	return JoinKey(prefix, name)
}

// Ext returns the lowercased extension of filename, including the dot, or ""
// when filename has none. Only the base name is considered.
func Ext(filename string) string {
	base := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	ext := strings.ToLower(path.Ext(base))
	if ext == "." || ext == base {
		return ""
	}
	return ext
}

// JoinURL appends the escaped key to base. A missing "/" between them is
// added. Each key segment is path-escaped, so the result is a valid URL
// even when the key contains spaces or non-ASCII characters.
func JoinURL(base, key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + strings.Join(segs, "/")
}
