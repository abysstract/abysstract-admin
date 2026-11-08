// Package localstorage is the filesystem storage driver, registered as
// "local". Objects live under a directory and are served by the driver's own
// HTTP handler, which the application mounts at MountPath (default
// "/static/").
//
// Presigned uploads are HMAC-signed PUT URLs to that same handler. They carry
// the key, content type, exact size and expiry, so the development flow
// matches the S3 flow: presign, PUT the bytes, confirm with Stat.
package localstorage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/abysstract/abysstract-admin/backend/storage"
)

// DefaultMountPath is where the application serves local objects.
const DefaultMountPath = "/static/"

// MinSecretLength is the shortest accepted signing secret, in bytes.
const MinSecretLength = 16

func init() {
	storage.Register("local", func(_ context.Context, cfg storage.Config) (storage.Storage, error) {
		return New(Options{
			Dir:           cfg.LocalDir,
			PublicBaseURL: cfg.PublicBaseURL,
			Secret:        cfg.SigningSecret,
		})
	})
}

// Options configures the local driver.
type Options struct {
	// Dir is the root directory. It is created if missing.
	Dir string
	// MountPath is the subtree the application mounts the handler at.
	// Default DefaultMountPath.
	MountPath string
	// PublicBaseURL prefixes keys in URL. Default MountPath, which yields
	// URLs relative to the API origin. Set it when a CDN or reverse proxy
	// serves Dir.
	PublicBaseURL string
	// UploadBaseURL prefixes keys in presigned URLs. Default MountPath. Set
	// an absolute URL when clients do not resolve relative URLs against the
	// API origin.
	UploadBaseURL string
	// Secret signs presigned uploads. PresignPut fails without it.
	Secret []byte
	// Now overrides the clock in tests.
	Now func() time.Time
}

// Storage is the local driver.
type Storage struct {
	root      *os.Root
	dir       string
	mountPath string
	publicURL string
	uploadURL string
	secret    []byte
	now       func() time.Time
}

var (
	_ storage.Storage     = (*Storage)(nil)
	_ storage.HTTPHandler = (*Storage)(nil)
)

// New creates the directory if needed and opens it as the storage root.
func New(opts Options) (*Storage, error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return nil, errors.New("localstorage: Dir is required")
	}
	if len(opts.Secret) > 0 && len(opts.Secret) < MinSecretLength {
		return nil, fmt.Errorf("localstorage: Secret must be at least %d bytes", MinSecretLength)
	}
	mount := opts.MountPath
	if mount == "" {
		mount = DefaultMountPath
	}
	if !strings.HasPrefix(mount, "/") || !strings.HasSuffix(mount, "/") || path.Clean(mount)+"/" != mount && mount != "/" {
		return nil, fmt.Errorf("localstorage: MountPath %q must be a clean path starting and ending with /", mount)
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("localstorage: create dir: %w", err)
	}
	root, err := os.OpenRoot(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("localstorage: open dir: %w", err)
	}
	s := &Storage{
		root:      root,
		dir:       opts.Dir,
		mountPath: mount,
		publicURL: firstNonEmpty(opts.PublicBaseURL, mount),
		uploadURL: firstNonEmpty(opts.UploadBaseURL, mount),
		secret:    append([]byte(nil), opts.Secret...),
		now:       opts.Now,
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// Dir returns the root directory.
func (s *Storage) Dir() string { return s.dir }

// MountPath implements storage.HTTPHandler.
func (s *Storage) MountPath() string { return s.mountPath }

// Close releases the directory handle.
func (s *Storage) Close() error { return s.root.Close() }

// URL implements storage.Storage.
func (s *Storage) URL(key string) string { return storage.JoinURL(s.publicURL, key) }

// Put writes to a temporary file in the target directory and renames it into
// place, so readers never see a partial object.
func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, _ string) error {
	if err := storage.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := path.Dir(key)
	if dir != "." {
		if err := s.root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("localstorage: create dir: %w", err)
		}
	}
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	// Keys cannot contain segments starting with ".", so temporary files
	// are never addressable through the handler.
	tmp := path.Join(dir, ".upload-"+hex.EncodeToString(rnd[:]))
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("localstorage: create: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
			_ = s.root.Remove(tmp)
		}
	}()
	src := r
	if size >= 0 {
		src = io.LimitReader(r, size+1)
	}
	n, err := io.Copy(f, readerWithContext{ctx: ctx, r: src})
	if err != nil {
		return fmt.Errorf("localstorage: write: %w", err)
	}
	if size >= 0 && n != size {
		return fmt.Errorf("%w: got %d bytes, want %d", storage.ErrSizeMismatch, n, size)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("localstorage: close: %w", err)
	}
	if err := s.root.Rename(tmp, key); err != nil {
		return fmt.Errorf("localstorage: rename: %w", err)
	}
	ok = true
	return nil
}

// Stat implements storage.Storage. The content type is derived from the key
// extension because the filesystem does not record it.
func (s *Storage) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.ObjectInfo{}, err
	}
	info, err := s.root.Stat(key)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ObjectInfo{}, fmt.Errorf("%w: %s", storage.ErrNotFound, key)
		}
		return storage.ObjectInfo{}, fmt.Errorf("localstorage: stat: %w", err)
	}
	if info.IsDir() {
		return storage.ObjectInfo{}, fmt.Errorf("%w: %s", storage.ErrNotFound, key)
	}
	return storage.ObjectInfo{
		Key:          key,
		Size:         info.Size(),
		ContentType:  storage.ContentTypeByExt(key),
		LastModified: info.ModTime(),
	}, nil
}

type readerWithContext struct {
	ctx context.Context
	r   io.Reader
}

func (r readerWithContext) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
