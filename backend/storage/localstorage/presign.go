package localstorage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/abysstract/abysstract-admin/backend/storage"
)

// Query parameters of a presigned upload URL.
const (
	paramExpires   = "X-Expires"
	paramSize      = "X-Size"
	paramSignature = "X-Signature"
)

// Presign verification errors.
var (
	ErrNoSecret         = errors.New("localstorage: presigned uploads need a signing secret")
	ErrBadSignature     = errors.New("localstorage: invalid upload signature")
	ErrExpired          = errors.New("localstorage: upload URL has expired")
	ErrContentType      = errors.New("localstorage: content type does not match the signed value")
	ErrContentLength    = errors.New("localstorage: content length does not match the signed size")
	errMissingParameter = errors.New("localstorage: missing upload parameter")
)

// PresignPut implements storage.Storage. The returned URL targets the
// driver's handler; the client must send the Content-Type header and a body
// of exactly size bytes before ExpiresAt.
func (s *Storage) PresignPut(_ context.Context, key, contentType string, size int64, ttl time.Duration) (storage.PresignedUpload, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.PresignedUpload{}, err
	}
	if len(s.secret) == 0 {
		return storage.PresignedUpload{}, ErrNoSecret
	}
	if size <= 0 {
		return storage.PresignedUpload{}, fmt.Errorf("localstorage: presign size must be positive (got %d)", size)
	}
	if ttl <= 0 {
		return storage.PresignedUpload{}, fmt.Errorf("localstorage: presign ttl must be positive (got %s)", ttl)
	}
	expires := s.now().Add(ttl).Truncate(time.Second)
	q := url.Values{}
	q.Set(paramExpires, strconv.FormatInt(expires.Unix(), 10))
	q.Set(paramSize, strconv.FormatInt(size, 10))
	q.Set(paramSignature, s.sign(key, contentType, size, expires.Unix()))
	return storage.PresignedUpload{
		URL:       storage.JoinURL(s.uploadURL, key) + "?" + q.Encode(),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: expires,
	}, nil
}

func (s *Storage) sign(key, contentType string, size, expires int64) string {
	mac := hmac.New(sha256.New, s.secret)
	// Fields are newline-separated; ValidateKey forbids control characters
	// in keys, and content types cannot contain newlines in an HTTP header.
	fmt.Fprintf(mac, "PUT\n%s\n%s\n%d\n%d", key, contentType, size, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verifiedUpload is a validated signed PUT.
type verifiedUpload struct {
	contentType string
	size        int64
}

// verify checks the signature, expiry, content type and declared length of a
// signed PUT request for key.
func (s *Storage) verify(key string, r *http.Request) (verifiedUpload, error) {
	if len(s.secret) == 0 {
		return verifiedUpload{}, ErrNoSecret
	}
	q := r.URL.Query()
	expRaw, sizeRaw, sig := q.Get(paramExpires), q.Get(paramSize), q.Get(paramSignature)
	if expRaw == "" || sizeRaw == "" || sig == "" {
		return verifiedUpload{}, errMissingParameter
	}
	expires, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil {
		return verifiedUpload{}, ErrBadSignature
	}
	size, err := strconv.ParseInt(sizeRaw, 10, 64)
	if err != nil || size <= 0 {
		return verifiedUpload{}, ErrBadSignature
	}
	contentType := r.Header.Get("Content-Type")
	want := s.sign(key, contentType, size, expires)
	if !hmac.Equal([]byte(sig), []byte(want)) {
		// A wrong Content-Type also lands here because it is part of the
		// signed string; report it as a signature failure.
		return verifiedUpload{}, ErrBadSignature
	}
	if !s.now().Before(time.Unix(expires, 0)) {
		return verifiedUpload{}, ErrExpired
	}
	if r.ContentLength != size {
		return verifiedUpload{}, ErrContentLength
	}
	return verifiedUpload{contentType: contentType, size: size}, nil
}
