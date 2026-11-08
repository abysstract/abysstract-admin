// Package storage is the replaceable object storage abstraction.
//
// Objects are addressed by keys: storage-agnostic relative paths such as
// "files/2026/11/3f9c.png". Applications store keys in their database and
// resolve URLs with [Storage.URL] when they build responses, so a domain or
// CDN change only touches configuration.
//
// Uploaders that must not hold bucket credentials get a short-lived presigned
// PUT from [Storage.PresignPut] and send the bytes straight to storage. The
// application then confirms the object with [VerifyUpload] before it records
// the key.
//
// Drivers live in subpackages: localstorage (filesystem, the default) and
// s3storage (S3-compatible services). A driver registers itself under a name
// so [Open] can build it from configuration; importing s3storage is what
// makes the "s3" driver available.
package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// ErrNotFound reports that no object exists under a key.
var ErrNotFound = errors.New("storage: object not found")

// Storage stores objects under keys. Implementations must be safe for
// concurrent use.
type Storage interface {
	// Put writes r to key, replacing any existing object. size is the exact
	// byte count, or -1 when unknown. contentType is recorded where the
	// backend supports it.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Stat returns object metadata. It returns an error wrapping ErrNotFound
	// when the key holds no object.
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// PresignPut authorizes one direct upload of exactly size bytes with the
	// given content type to key, valid for ttl.
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (PresignedUpload, error)
	// URL returns the public URL of key. It does not check that the object
	// exists.
	URL(key string) string
}

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

// PresignedUpload is everything a client needs to upload directly.
// The client sends Method to URL with Headers set exactly as given and a body
// of the presigned size. URL may be relative to the API origin (local driver).
type PresignedUpload struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

// HTTPHandler is implemented by drivers that serve objects or signed uploads
// from the application itself. The application mounts the handler at
// MountPath, a subtree path such as "/static/".
type HTTPHandler interface {
	http.Handler
	MountPath() string
}
