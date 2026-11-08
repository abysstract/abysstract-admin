package s3storage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/abysstract/abysstract-admin/backend/storage"
	"github.com/abysstract/abysstract-admin/backend/storage/s3storage"
)

// fakeS3 implements the path-style PUT and HEAD object calls the driver uses.
// It does not verify signatures; the tests check signed parameters directly.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string]fakeObject
	puts    []*http.Request
}

type fakeObject struct {
	body        []byte
	contentType string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.ContentLength >= 0 && int64(len(body)) != r.ContentLength {
			http.Error(w, "length mismatch", http.StatusBadRequest)
			return
		}
		f.puts = append(f.puts, r.Clone(context.Background()))
		f.objects[r.URL.Path] = fakeObject{body: body, contentType: r.Header.Get("Content-Type")}
		w.Header().Set("ETag", `"etag-1"`)
	case http.MethodHead:
		obj, ok := f.objects[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(obj.body)))
		w.Header().Set("Content-Type", obj.contentType)
		w.Header().Set("ETag", `"etag-1"`)
		w.Header().Set("Last-Modified", time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func newFake(t *testing.T) (*fakeS3, *s3storage.Storage, string) {
	t.Helper()
	fake := &fakeS3{objects: map[string]fakeObject{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	st, err := s3storage.New(context.Background(), s3storage.Options{
		Bucket:       "media",
		Region:       "us-east-1",
		Endpoint:     srv.URL,
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", ""),
		Now:          func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return fake, st, srv.URL
}

func TestOptionsValidation(t *testing.T) {
	ctx := context.Background()
	for _, opts := range []s3storage.Options{
		{Region: "us-east-1"},
		{Bucket: "b"},
		{Bucket: "b", Region: "us-east-1", Endpoint: "localhost:9000"},
	} {
		if _, err := s3storage.New(ctx, opts); err == nil {
			t.Errorf("New(%+v) accepted", opts)
		}
	}
	if _, err := s3storage.NewWithClient(nil, s3storage.Options{Bucket: "b", Region: "r"}); err == nil {
		t.Error("nil client accepted")
	}
}

func TestDefaultPublicURL(t *testing.T) {
	creds := credentials.NewStaticCredentialsProvider("a", "b", "")
	cases := []struct {
		opts s3storage.Options
		want string
	}{
		{s3storage.Options{Bucket: "media", Region: "eu-west-1"}, "https://media.s3.eu-west-1.amazonaws.com/a/b.png"},
		{s3storage.Options{Bucket: "media", Region: "eu-west-1", UsePathStyle: true}, "https://s3.eu-west-1.amazonaws.com/media/a/b.png"},
		{s3storage.Options{Bucket: "media", Region: "us-east-1", Endpoint: "http://localhost:9000", UsePathStyle: true}, "http://localhost:9000/media/a/b.png"},
		{s3storage.Options{Bucket: "media", Region: "auto", Endpoint: "https://acct.r2.example.com"}, "https://media.acct.r2.example.com/a/b.png"},
		{s3storage.Options{Bucket: "media", Region: "auto", PublicBaseURL: "https://img.example.com"}, "https://img.example.com/a/b.png"},
	}
	for _, c := range cases {
		c.opts.Credentials = creds
		st, err := s3storage.New(context.Background(), c.opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.URL("a/b.png"); got != c.want {
			t.Errorf("URL with %+v = %q, want %q", c.opts, got, c.want)
		}
	}
}

func TestPutAndStat(t *testing.T) {
	fake, st, _ := newFake(t)
	ctx := context.Background()
	if err := st.Put(ctx, "files/a.png", bytes.NewReader([]byte("hello")), 5, "image/png"); err != nil {
		t.Fatal(err)
	}
	// A reader that cannot seek goes out with an unsigned payload.
	if err := st.Put(ctx, "files/b.txt", io.MultiReader(strings.NewReader("abc")), 3, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if got := fake.objects["/media/files/a.png"]; string(got.body) != "hello" || got.contentType != "image/png" {
		t.Fatalf("stored = %+v", got)
	}
	if got := fake.puts[1].Header.Get("X-Amz-Content-Sha256"); got != "UNSIGNED-PAYLOAD" {
		t.Fatalf("non-seekable payload hash header = %q", got)
	}
	info, err := st.Stat(ctx, "files/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != 5 || info.ContentType != "image/png" || info.ETag != "etag-1" || info.LastModified.IsZero() {
		t.Fatalf("Stat = %+v", info)
	}
	if _, err := st.Stat(ctx, "files/missing.png"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if err := st.Put(ctx, "../x", strings.NewReader("x"), 1, ""); !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("bad key = %v", err)
	}
}

func TestPresignPut(t *testing.T) {
	fake, st, endpoint := newFake(t)
	ctx := context.Background()
	up, err := st.PresignPut(ctx, "files/x.png", "image/png", 4, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if up.Method != http.MethodPut {
		t.Fatalf("method = %s", up.Method)
	}
	if want := time.Date(2026, 11, 8, 12, 10, 0, 0, time.UTC); !up.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v", up.ExpiresAt)
	}
	u, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(up.URL, endpoint+"/media/files/x.png?") {
		t.Fatalf("URL = %s", up.URL)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "600" || q.Get("X-Amz-Signature") == "" {
		t.Fatalf("query = %v", q)
	}
	signed := strings.Split(q.Get("X-Amz-SignedHeaders"), ";")
	for _, h := range []string{"content-length", "content-type", "host"} {
		if !contains(signed, h) {
			t.Fatalf("signed headers %v missing %s", signed, h)
		}
	}
	for name := range q {
		if strings.Contains(strings.ToLower(name), "checksum") {
			t.Fatalf("presigned URL requires a checksum: %s", name)
		}
	}
	if up.Headers["Content-Type"] != "image/png" {
		t.Fatalf("headers = %v", up.Headers)
	}
	if _, ok := up.Headers["Host"]; ok {
		t.Fatalf("headers include Host: %v", up.Headers)
	}

	req, _ := http.NewRequest(up.Method, up.URL, bytes.NewReader([]byte("data")))
	for k, v := range up.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	if _, err := storage.VerifyUpload(ctx, st, "files/x.png", 4); err != nil {
		t.Fatal(err)
	}
	if string(fake.objects["/media/files/x.png"].body) != "data" {
		t.Fatal("object not stored")
	}

	for _, ttl := range []time.Duration{0, s3storage.MaxPresignTTL + time.Second} {
		if _, err := st.PresignPut(ctx, "files/x.png", "image/png", 4, ttl); err == nil {
			t.Errorf("ttl %s accepted", ttl)
		}
	}
	if _, err := st.PresignPut(ctx, "files/x.png", "image/png", 0, time.Minute); err == nil {
		t.Error("zero size accepted")
	}
}

func TestRegisteredAsS3(t *testing.T) {
	if !storage.Registered("s3") {
		t.Fatal("s3 driver not registered")
	}
	_, err := storage.Open(context.Background(), storage.Config{Driver: "s3"})
	if err == nil || !strings.Contains(err.Error(), "Bucket") {
		t.Fatalf("Open without bucket = %v", err)
	}
}

// TestLiveS3 runs against a real S3-compatible service, for example MinIO:
//
//	ADMIN_TEST_S3_ENDPOINT=http://localhost:9000 ADMIN_TEST_S3_BUCKET=test \
//	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
//	go test ./backend/storage/s3storage -run TestLiveS3
//
// The bucket must exist. The test is skipped when no endpoint is configured.
func TestLiveS3(t *testing.T) {
	endpoint := os.Getenv("ADMIN_TEST_S3_ENDPOINT")
	bucket := os.Getenv("ADMIN_TEST_S3_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("set ADMIN_TEST_S3_ENDPOINT and ADMIN_TEST_S3_BUCKET to run against a live S3-compatible service")
	}
	region := os.Getenv("ADMIN_TEST_S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	ctx := context.Background()
	st, err := s3storage.New(ctx, s3storage.Options{Bucket: bucket, Region: region, Endpoint: endpoint, UsePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	prefix := "abysstract-test/" + hex.EncodeToString(rnd[:])

	if err := st.Put(ctx, prefix+"/direct.txt", strings.NewReader("direct"), 6, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if info, err := st.Stat(ctx, prefix+"/direct.txt"); err != nil || info.Size != 6 {
		t.Fatalf("Stat = %+v, %v", info, err)
	}
	if _, err := st.Stat(ctx, prefix+"/missing.txt"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}

	body := []byte("presigned-body")
	up, err := st.PresignPut(ctx, prefix+"/presigned.png", "image/png", int64(len(body)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	send := func(contentType string, payload []byte) int {
		req, _ := http.NewRequest(up.Method, up.URL, bytes.NewReader(payload))
		for k, v := range up.Headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Type", contentType)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := send("text/html", body); code == http.StatusOK {
		t.Fatal("service accepted a different content type")
	}
	if code := send("image/png", append(body, '!')); code == http.StatusOK {
		t.Fatal("service accepted a different length")
	}
	if code := send("image/png", body); code != http.StatusOK {
		t.Fatalf("presigned PUT = %d", code)
	}
	if _, err := storage.VerifyUpload(ctx, st, prefix+"/presigned.png", int64(len(body))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, key := range []string{prefix + "/direct.txt", prefix + "/presigned.png"} {
			_, _ = st.Client().DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(st.Bucket()), Key: aws.String(key)})
		}
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
