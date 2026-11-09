package storage

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"
	"time"
)

// Upload policy errors. Check wraps them, so callers can match with errors.Is.
var (
	ErrEmpty         = errors.New("storage: file is empty")
	ErrTooLarge      = errors.New("storage: file exceeds the size limit")
	ErrExtNotAllowed = errors.New("storage: file type is not allowed")
	ErrSizeMismatch  = errors.New("storage: uploaded size does not match")
)

// DefaultUploadTTL is how long a presigned upload stays valid when the
// caller does not choose.
const DefaultUploadTTL = 15 * time.Minute

// ImageExts returns the image extensions accepted by the bundled avatar
// upload. Each call returns a new slice.
func ImageExts() []string {
	return []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
}

// FileExts returns the extensions accepted by the bundled file upload.
// Each call returns a new slice.
func FileExts() []string {
	return append(ImageExts(),
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".txt", ".csv", ".zip",
	)
}

// UploadPolicy is the size and extension rule applied before an upload is
// accepted or presigned.
type UploadPolicy struct {
	// MaxBytes is the largest accepted size. It must be positive.
	MaxBytes int64
	// AllowedExts lists lowercased extensions including the dot, such as
	// ".png". An empty list rejects every file.
	AllowedExts []string
}

// Check validates filename and size against the policy.
func (p UploadPolicy) Check(filename string, size int64) error {
	if size <= 0 {
		return ErrEmpty
	}
	if size > p.MaxBytes {
		return fmt.Errorf("%w: %d > %d", ErrTooLarge, size, p.MaxBytes)
	}
	ext := Ext(filename)
	for _, allowed := range p.AllowedExts {
		if ext != "" && strings.EqualFold(ext, allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrExtNotAllowed, ext)
}

// ContentTypeByExt returns the MIME type for filename's extension, or
// "application/octet-stream" when unknown. Servers should derive the
// content type this way rather than trust the client's claim.
func ContentTypeByExt(filename string) string {
	if ct := mime.TypeByExtension(Ext(filename)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// PresignRequest describes one direct upload an application wants to
// authorize.
type PresignRequest struct {
	// Key is where the object will be stored. Applications choose it, for
	// example with RandomKey or a content-addressed rule.
	Key string
	// Filename is the client's original name; only its extension is used.
	Filename string
	// Size is the exact byte count the client will send.
	Size int64
	// TTL defaults to DefaultUploadTTL.
	TTL time.Duration
}

// PresignResult is the outcome of Presign.
type PresignResult struct {
	Key         string
	ContentType string
	// URL is where the object will be readable once uploaded.
	URL    string
	Upload PresignedUpload
}

// Presign checks req against policy, derives the content type from the file
// extension and asks st for a presigned PUT. Applications use it to build
// their own presign endpoints with their own key rules.
func Presign(ctx context.Context, st Storage, policy UploadPolicy, req PresignRequest) (PresignResult, error) {
	if err := policy.Check(req.Filename, req.Size); err != nil {
		return PresignResult{}, err
	}
	if err := ValidateKey(req.Key); err != nil {
		return PresignResult{}, err
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = DefaultUploadTTL
	}
	ct := ContentTypeByExt(req.Filename)
	up, err := st.PresignPut(ctx, req.Key, ct, req.Size, ttl)
	if err != nil {
		return PresignResult{}, err
	}
	return PresignResult{Key: req.Key, ContentType: ct, URL: st.URL(req.Key), Upload: up}, nil
}

// VerifyUpload confirms that a presigned upload has landed. It returns the
// object metadata, an error wrapping ErrNotFound when nothing was uploaded,
// or ErrSizeMismatch when wantSize is positive and differs from the stored
// size. Call it before recording a client-supplied key.
func VerifyUpload(ctx context.Context, st Storage, key string, wantSize int64) (ObjectInfo, error) {
	if err := ValidateKey(key); err != nil {
		return ObjectInfo{}, err
	}
	info, err := st.Stat(ctx, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if wantSize > 0 && info.Size != wantSize {
		return info, fmt.Errorf("%w: got %d, want %d", ErrSizeMismatch, info.Size, wantSize)
	}
	return info, nil
}
