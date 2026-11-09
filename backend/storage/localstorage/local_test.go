package localstorage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/storage"
	"github.com/abysstract/abysstract-admin/backend/storage/localstorage"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStorage(t *testing.T, opts localstorage.Options) (*localstorage.Storage, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	if opts.Dir == "" {
		opts.Dir = t.TempDir()
	}
	if opts.Secret == nil {
		opts.Secret = secret
	}
	opts.Now = c.now
	st, err := localstorage.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, c
}

func TestNewValidation(t *testing.T) {
	if _, err := localstorage.New(localstorage.Options{}); err == nil {
		t.Fatal("missing Dir accepted")
	}
	if _, err := localstorage.New(localstorage.Options{Dir: t.TempDir(), Secret: []byte("short")}); err == nil {
		t.Fatal("short secret accepted")
	}
	for _, mount := range []string{"static/", "/static", "/a//b/", "/a/../b/"} {
		if _, err := localstorage.New(localstorage.Options{Dir: t.TempDir(), MountPath: mount}); err == nil {
			t.Errorf("MountPath %q accepted", mount)
		}
	}
}

func TestPutStatURL(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	if err := st.Put(ctx, "files/2026/a.png", strings.NewReader("hello"), 5, "image/png"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(st.Dir(), "files", "2026", "a.png"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("file = %q, %v", b, err)
	}
	info, err := st.Stat(ctx, "files/2026/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != 5 || info.ContentType != "image/png" || info.Key != "files/2026/a.png" {
		t.Fatalf("Stat = %+v", info)
	}
	if _, err := st.Stat(ctx, "files/2026/missing.png"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing Stat = %v", err)
	}
	if _, err := st.Stat(ctx, "files/2026"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("directory Stat = %v", err)
	}
	if got := st.URL("files/2026/a b.png"); got != "/static/files/2026/a%20b.png" {
		t.Fatalf("URL = %q", got)
	}
	// Unknown size is allowed.
	if err := st.Put(ctx, "b.txt", strings.NewReader("abc"), -1, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPutRejectsBadInput(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	if err := st.Put(ctx, "../escape.txt", strings.NewReader("x"), 1, ""); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("escape = %v", err)
	}
	if err := st.Put(ctx, "short.txt", strings.NewReader("abc"), 5, ""); !errors.Is(err, storage.ErrSizeMismatch) {
		t.Fatalf("short = %v", err)
	}
	if err := st.Put(ctx, "long.txt", strings.NewReader("abcdef"), 5, ""); !errors.Is(err, storage.ErrSizeMismatch) {
		t.Fatalf("long = %v", err)
	}
	entries, _ := os.ReadDir(st.Dir())
	if len(entries) != 0 {
		t.Fatalf("failed puts left files: %v", entries)
	}
}

func TestPutDoesNotFollowSymlinkOutOfRoot(t *testing.T) {
	outside := t.TempDir()
	st, _ := newStorage(t, localstorage.Options{})
	if err := os.Symlink(outside, filepath.Join(st.Dir(), "link")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := st.Put(context.Background(), "link/x.txt", strings.NewReader("x"), 1, ""); err == nil {
		t.Fatal("Put escaped the root through a symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "x.txt")); err == nil {
		t.Fatal("file written outside the root")
	}
}

func TestCustomBaseURLs(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{
		PublicBaseURL: "https://cdn.example.com/media",
		UploadBaseURL: "https://api.example.com/static/",
	})
	if got := st.URL("a.png"); got != "https://cdn.example.com/media/a.png" {
		t.Fatalf("URL = %q", got)
	}
	up, err := st.PresignPut(context.Background(), "a.png", "image/png", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(up.URL, "https://api.example.com/static/a.png?") {
		t.Fatalf("presign URL = %q", up.URL)
	}
}

func put(t *testing.T, h http.Handler, rawURL string, contentType string, body []byte, contentLength int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, rawURL, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = contentLength
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPresignedUploadRoundTrip(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	body := []byte("png-bytes")
	up, err := st.PresignPut(ctx, "files/x.png", "image/png", int64(len(body)), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if up.Method != http.MethodPut || up.Headers["Content-Type"] != "image/png" {
		t.Fatalf("presign = %+v", up)
	}
	if want := time.Date(2026, 11, 8, 12, 10, 0, 0, time.UTC); !up.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", up.ExpiresAt, want)
	}
	rec := put(t, st, up.URL, "image/png", body, int64(len(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	info, err := storage.VerifyUpload(ctx, st, "files/x.png", int64(len(body)))
	if err != nil || info.Size != int64(len(body)) {
		t.Fatalf("VerifyUpload = %+v, %v", info, err)
	}

	get := httptest.NewRecorder()
	st.ServeHTTP(get, httptest.NewRequest(http.MethodGet, st.URL("files/x.png"), nil))
	if get.Code != http.StatusOK || get.Body.String() != string(body) {
		t.Fatalf("GET = %d %q", get.Code, get.Body)
	}
	if get.Header().Get("X-Content-Type-Options") != "nosniff" || get.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("GET headers = %v", get.Header())
	}
}

func TestPresignedUploadRejections(t *testing.T) {
	st, clk := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	body := []byte("12345")
	up, err := st.PresignPut(ctx, "files/x.png", "image/png", 5, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tamper := func(param, value string) string {
		u, _ := url.Parse(up.URL)
		q := u.Query()
		q.Set(param, value)
		u.RawQuery = q.Encode()
		return u.String()
	}
	otherKey := strings.Replace(up.URL, "files/x.png", "files/y.png", 1)

	cases := []struct {
		name   string
		url    string
		ct     string
		body   []byte
		length int64
		want   int
	}{
		{"wrong content type", up.URL, "text/html", body, 5, http.StatusForbidden},
		{"other key", otherKey, "image/png", body, 5, http.StatusForbidden},
		{"bigger size", tamper("X-Size", "6"), "image/png", append(body, 'x'), 6, http.StatusForbidden},
		{"later expiry", tamper("X-Expires", "9999999999"), "image/png", body, 5, http.StatusForbidden},
		{"bad signature", tamper("X-Signature", "AAAA"), "image/png", body, 5, http.StatusForbidden},
		{"missing signature", strings.Split(up.URL, "?")[0], "image/png", body, 5, http.StatusBadRequest},
		{"wrong length", up.URL, "image/png", []byte("123456"), 6, http.StatusBadRequest},
		{"chunked", up.URL, "image/png", body, -1, http.StatusBadRequest},
		{"short body", up.URL, "image/png", []byte("123"), 5, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := put(t, st, c.url, c.ct, c.body, c.length)
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, c.want, rec.Body)
			}
			if _, err := st.Stat(ctx, "files/x.png"); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("rejected upload stored an object: %v", err)
			}
		})
	}

	clk.t = clk.t.Add(time.Minute)
	if rec := put(t, st, up.URL, "image/png", body, 5); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "expired") {
		t.Fatalf("expired = %d %s", rec.Code, rec.Body)
	}
	clk.t = clk.t.Add(-time.Second)
	if rec := put(t, st, up.URL, "image/png", body, 5); rec.Code != http.StatusOK {
		t.Fatalf("just before expiry = %d %s", rec.Code, rec.Body)
	}
}

func TestPresignValidation(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	if _, err := st.PresignPut(ctx, "a.png", "image/png", 0, time.Minute); err == nil {
		t.Fatal("zero size accepted")
	}
	if _, err := st.PresignPut(ctx, "a.png", "image/png", 1, 0); err == nil {
		t.Fatal("zero ttl accepted")
	}
	if _, err := st.PresignPut(ctx, "/a.png", "image/png", 1, time.Minute); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("bad key = %v", err)
	}
	noSecret, err := localstorage.New(localstorage.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := noSecret.PresignPut(ctx, "a.png", "image/png", 1, time.Minute); !errors.Is(err, localstorage.ErrNoSecret) {
		t.Fatalf("no secret = %v", err)
	}
}

func TestSignatureDependsOnSecret(t *testing.T) {
	dir := t.TempDir()
	a, _ := newStorage(t, localstorage.Options{Dir: dir})
	b, _ := newStorage(t, localstorage.Options{Dir: dir, Secret: []byte("another-secret-of-enough-length")})
	up, err := a.PresignPut(context.Background(), "a.png", "image/png", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if rec := put(t, b, up.URL, "image/png", []byte("x"), 1); rec.Code != http.StatusForbidden {
		t.Fatalf("other secret accepted: %d", rec.Code)
	}
}

func TestServeHTTP(t *testing.T) {
	st, _ := newStorage(t, localstorage.Options{})
	ctx := context.Background()
	for key, body := range map[string]string{"page.html": "<script>", "doc.pdf": "%PDF"} {
		if err := st.Put(ctx, key, strings.NewReader(body), int64(len(body)), ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), ".upload-tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, path string
		want         int
		csp          bool
	}{
		{http.MethodGet, "/static/page.html", http.StatusOK, true},
		{http.MethodHead, "/static/page.html", http.StatusOK, true},
		{http.MethodGet, "/static/doc.pdf", http.StatusOK, false},
		{http.MethodGet, "/static/", http.StatusNotFound, false},
		{http.MethodGet, "/static/missing.png", http.StatusNotFound, false},
		{http.MethodGet, "/static/.upload-tmp", http.StatusNotFound, false},
		{http.MethodGet, "/other/page.html", http.StatusNotFound, false},
		{http.MethodDelete, "/static/page.html", http.StatusMethodNotAllowed, false},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		st.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.want {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, rec.Code, c.want)
			continue
		}
		if c.want == http.StatusOK {
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s missing nosniff", c.path)
			}
			if got := rec.Header().Get("Content-Security-Policy") != ""; got != c.csp {
				t.Errorf("%s CSP present = %v, want %v", c.path, got, c.csp)
			}
		}
	}
}

func TestRegisteredAsLocal(t *testing.T) {
	st, err := storage.Open(context.Background(), storage.Config{Driver: "local", LocalDir: t.TempDir(), SigningSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.(storage.HTTPHandler); !ok {
		t.Fatal("local driver does not implement HTTPHandler")
	}
	if closer, ok := st.(io.Closer); ok {
		_ = closer.Close()
	}
}
