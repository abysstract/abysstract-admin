package storage_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/storage"
)

func TestValidateKey(t *testing.T) {
	valid := []string{
		"a", "a.png", "files/2026/11/abc.png", "avatars/x_y-z.webp", "中文/图片.jpg", "a b/c d.txt",
	}
	for _, key := range valid {
		if err := storage.ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", key, err)
		}
	}
	invalid := []string{
		"", "/abs", "/", "a//b", "a/", "../x", "a/../b", "a/./b", ".", "..", ".hidden", "a/.env",
		`a\b`, "a\x00b", "a\nb", strings.Repeat("a", storage.MaxKeyLength+1),
	}
	for _, key := range invalid {
		err := storage.ValidateKey(key)
		if !errors.Is(err, storage.ErrInvalidKey) {
			t.Errorf("ValidateKey(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}

func TestJoinKey(t *testing.T) {
	got, err := storage.JoinKey("files", "2026/11", "a.png")
	if err != nil || got != "files/2026/11/a.png" {
		t.Fatalf("JoinKey = %q, %v", got, err)
	}
	if _, err := storage.JoinKey("files", "", "a.png"); err == nil {
		t.Fatal("JoinKey accepted an empty part")
	}
	if _, err := storage.JoinKey("files", "../a.png"); err == nil {
		t.Fatal("JoinKey accepted ..")
	}
}

func TestRandomKey(t *testing.T) {
	a, err := storage.RandomKey("files/2026", `C:\Users\x\Photo.JPG`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "files/2026/") || !strings.HasSuffix(a, ".jpg") || len(a) != len("files/2026/")+32+4 {
		t.Fatalf("RandomKey = %q", a)
	}
	b, _ := storage.RandomKey("files/2026", "photo.jpg")
	if a == b {
		t.Fatal("RandomKey repeated")
	}
	c, err := storage.RandomKey("", "noext")
	if err != nil || len(c) != 32 {
		t.Fatalf("RandomKey without prefix = %q, %v", c, err)
	}
	if _, err := storage.RandomKey("/abs", "a.png"); err == nil {
		t.Fatal("RandomKey accepted an absolute prefix")
	}
}

func TestExt(t *testing.T) {
	cases := map[string]string{
		"a.PNG": ".png", "dir/a.tar.gz": ".gz", ".png": "", "noext": "", "a.": "", `x\y.Doc`: ".doc", "a.png/": ".png",
	}
	for in, want := range cases {
		if got := storage.Ext(in); got != want {
			t.Errorf("Ext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJoinURL(t *testing.T) {
	cases := []struct{ base, key, want string }{
		{"/static/", "a/b.png", "/static/a/b.png"},
		{"https://cdn.example.com", "a b/c#d.png", "https://cdn.example.com/a%20b/c%23d.png"},
		{"https://cdn.example.com/media/", "x?.png", "https://cdn.example.com/media/x%3F.png"},
	}
	for _, c := range cases {
		if got := storage.JoinURL(c.base, c.key); got != c.want {
			t.Errorf("JoinURL(%q, %q) = %q, want %q", c.base, c.key, got, c.want)
		}
	}
}

func TestUploadPolicyCheck(t *testing.T) {
	p := storage.UploadPolicy{MaxBytes: 10, AllowedExts: []string{".png", ".JPG"}}
	if err := p.Check("a.png", 10); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if err := p.Check("a.jpg", 1); err != nil {
		t.Fatalf("Check upper-case allow-list: %v", err)
	}
	for _, c := range []struct {
		name string
		size int64
		want error
	}{
		{"a.png", 0, storage.ErrEmpty},
		{"a.png", 11, storage.ErrTooLarge},
		{"a.exe", 1, storage.ErrExtNotAllowed},
		{"png", 1, storage.ErrExtNotAllowed},
	} {
		if err := p.Check(c.name, c.size); !errors.Is(err, c.want) {
			t.Errorf("Check(%q, %d) = %v, want %v", c.name, c.size, err, c.want)
		}
	}
}

func TestContentTypeByExt(t *testing.T) {
	if got := storage.ContentTypeByExt("a.PNG"); got != "image/png" {
		t.Fatalf("png = %q", got)
	}
	if got := storage.ContentTypeByExt("a.unknownext"); got != "application/octet-stream" {
		t.Fatalf("unknown = %q", got)
	}
}

type memStorage struct {
	objects map[string][]byte
	presign []string
}

func (m *memStorage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(r)
	m.objects[key] = b
	return err
}

func (m *memStorage) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	b, ok := m.objects[key]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	return storage.ObjectInfo{Key: key, Size: int64(len(b))}, nil
}

func (m *memStorage) PresignPut(_ context.Context, key, ct string, size int64, ttl time.Duration) (storage.PresignedUpload, error) {
	m.presign = append(m.presign, key+"|"+ct)
	return storage.PresignedUpload{URL: "https://up/" + key, Method: "PUT", ExpiresAt: time.Unix(0, 0).Add(ttl)}, nil
}

func (m *memStorage) URL(key string) string { return "https://cdn/" + key }

func TestPresign(t *testing.T) {
	st := &memStorage{objects: map[string][]byte{}}
	policy := storage.UploadPolicy{MaxBytes: 100, AllowedExts: []string{".png"}}
	res, err := storage.Presign(context.Background(), st, policy, storage.PresignRequest{Key: "img/a.png", Filename: "x.png", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.Key != "img/a.png" || res.ContentType != "image/png" || res.URL != "https://cdn/img/a.png" || res.Upload.URL != "https://up/img/a.png" {
		t.Fatalf("Presign = %+v", res)
	}
	if res.Upload.ExpiresAt != time.Unix(0, 0).Add(storage.DefaultUploadTTL) {
		t.Fatalf("default ttl not applied: %v", res.Upload.ExpiresAt)
	}
	if _, err := storage.Presign(context.Background(), st, policy, storage.PresignRequest{Key: "img/a.png", Filename: "x.exe", Size: 5}); !errors.Is(err, storage.ErrExtNotAllowed) {
		t.Fatalf("ext err = %v", err)
	}
	if _, err := storage.Presign(context.Background(), st, policy, storage.PresignRequest{Key: "../a.png", Filename: "x.png", Size: 5}); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("key err = %v", err)
	}
	if len(st.presign) != 1 {
		t.Fatalf("rejected requests reached the driver: %v", st.presign)
	}
}

func TestVerifyUpload(t *testing.T) {
	st := &memStorage{objects: map[string][]byte{"a.png": []byte("hello")}}
	ctx := context.Background()
	if _, err := storage.VerifyUpload(ctx, st, "a.png", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.VerifyUpload(ctx, st, "a.png", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.VerifyUpload(ctx, st, "a.png", 6); !errors.Is(err, storage.ErrSizeMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}
	if _, err := storage.VerifyUpload(ctx, st, "b.png", 5); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := storage.VerifyUpload(ctx, st, "/etc/passwd", 5); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("invalid err = %v", err)
	}
}

func TestRegistry(t *testing.T) {
	storage.Register("test-mem", func(context.Context, storage.Config) (storage.Storage, error) {
		return &memStorage{objects: map[string][]byte{}}, nil
	})
	if !storage.Registered("test-mem") {
		t.Fatal("not registered")
	}
	if _, err := storage.Open(context.Background(), storage.Config{Driver: "test-mem"}); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Open(context.Background(), storage.Config{Driver: "nope"}); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Open unknown = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate Register did not panic")
		}
	}()
	storage.Register("test-mem", func(context.Context, storage.Config) (storage.Storage, error) { return nil, nil })
}
