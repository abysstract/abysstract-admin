package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/storage"
)

// memoryStorage 记录所有写入与预签名调用，便于断言「校验失败时不写入」。
type memoryStorage struct {
	objects  map[string][]byte
	types    map[string]string
	presigns []string
	fail     bool
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{objects: map[string][]byte{}, types: map[string]string{}}
}

func (m *memoryStorage) Put(_ context.Context, key string, r io.Reader, _ int64, contentType string) error {
	if m.fail {
		return errors.New("storage injected failure")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[key] = b
	m.types[key] = contentType
	return nil
}

func (m *memoryStorage) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	b, ok := m.objects[key]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	return storage.ObjectInfo{Key: key, Size: int64(len(b))}, nil
}

func (m *memoryStorage) PresignPut(_ context.Context, key, contentType string, size int64, ttl time.Duration) (storage.PresignedUpload, error) {
	if m.fail {
		return storage.PresignedUpload{}, errors.New("storage injected failure")
	}
	m.presigns = append(m.presigns, key)
	return storage.PresignedUpload{
		URL:       "https://upload.example.com/" + key,
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC).Add(ttl),
	}, nil
}

func (m *memoryStorage) URL(key string) string { return "https://cdn.example.com/" + key }

func newTestUploadService(st storage.Storage) *UploadService {
	return NewUploadService(nil, st, UploadConfig{
		FileMaxBytes:   100,
		AvatarMaxBytes: 100,
		UploadTTL:      5 * time.Minute,
		Now:            func() time.Time { return time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC) },
	})
}

// 属性 13：上传校验
// 对任意（大小、扩展）组合，接受 ⇔ 大小在 (0, 上限] 且扩展在白名单内；
// 被拒绝时不写入存储，并返回 InvalidArgument。
func TestPropertyUploadValidation(t *testing.T) {
	const maxBytes = 100
	allowed := map[string]bool{}
	for _, ext := range storage.FileExts() {
		allowed[ext] = true
	}
	check := func(size int, ext string) bool {
		st := newMemoryStorage()
		svc := newTestUploadService(st)
		_, err := svc.UploadFile(context.Background(), &adminv1.UploadFileRequest{
			Filename: "f" + ext, Content: make([]byte, size),
		})
		accepted := err == nil
		want := size > 0 && size <= maxBytes && allowed[strings.ToLower(ext)]
		if accepted != want {
			t.Errorf("size=%d ext=%q accepted=%v want=%v (%v)", size, ext, accepted, want, err)
			return false
		}
		if !accepted {
			if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
				t.Errorf("size=%d ext=%q err=%v, want InvalidArgument", size, ext, err)
				return false
			}
			if len(st.objects) != 0 {
				t.Errorf("rejected upload was stored")
				return false
			}
		}
		return true
	}
	sizes := []int{0, 1, 50, 99, 100, 101, 1000}
	exts := []string{".png", ".pdf", ".zip", ".exe", ".html", ".svg", "", ".PNG"}
	for _, sz := range sizes {
		for _, ex := range exts {
			if !check(sz, ex) {
				t.FailNow()
			}
		}
	}
	rngExts := []string{".png", ".pdf", ".zip", ".exe", ".html"}
	if err := quick.Check(func(size int, extIdx byte) bool {
		sz := size % (maxBytes + 50)
		if sz < 0 {
			sz = -sz
		}
		return check(sz, rngExts[int(extIdx)%len(rngExts)])
	}, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("quick check failed: %v", err)
	}
}

func TestUploadFileStoresUnderKey(t *testing.T) {
	st := newMemoryStorage()
	svc := newTestUploadService(st)
	resp, err := svc.UploadFile(context.Background(), &adminv1.UploadFileRequest{Filename: "Photo.PNG", Content: []byte("hello")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Key, "files/2026/03/") || !strings.HasSuffix(resp.Key, ".png") {
		t.Fatalf("key = %q", resp.Key)
	}
	if resp.Url != "https://cdn.example.com/"+resp.Key || resp.Size != 5 {
		t.Fatalf("resp = %+v", resp)
	}
	if string(st.objects[resp.Key]) != "hello" || st.types[resp.Key] != "image/png" {
		t.Fatalf("stored = %q %q", st.objects[resp.Key], st.types[resp.Key])
	}
}

func TestUploadFileStorageFailure(t *testing.T) {
	st := newMemoryStorage()
	st.fail = true
	_, err := newTestUploadService(st).UploadFile(context.Background(), &adminv1.UploadFileRequest{Filename: "a.png", Content: []byte("x")})
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInternal {
		t.Fatalf("err = %v, want Internal", err)
	}
}

func TestUploadAvatarRequiresLogin(t *testing.T) {
	st := newMemoryStorage()
	_, err := newTestUploadService(st).UploadAvatar(context.Background(), &adminv1.UploadAvatarRequest{Filename: "a.png", Content: []byte("x")})
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeUnauthenticated {
		t.Fatalf("err = %v, want Unauthenticated", err)
	}
	if len(st.objects) != 0 {
		t.Fatal("unauthenticated avatar was stored")
	}
}

func TestCreateUpload(t *testing.T) {
	st := newMemoryStorage()
	svc := newTestUploadService(st)
	resp, err := svc.CreateUpload(context.Background(), &adminv1.CreateUploadRequest{Filename: "report.pdf", ContentType: "text/html", Size: 42})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Key, "files/2026/03/") || !strings.HasSuffix(resp.Key, ".pdf") {
		t.Fatalf("key = %q", resp.Key)
	}
	if resp.Url != "https://cdn.example.com/"+resp.Key || resp.UploadUrl != "https://upload.example.com/"+resp.Key || resp.Method != "PUT" {
		t.Fatalf("resp = %+v", resp)
	}
	// The server derives the content type from the extension, not the claim.
	if resp.Headers["Content-Type"] != "application/pdf" {
		t.Fatalf("headers = %v", resp.Headers)
	}
	if got := resp.ExpiresAt.AsTime(); !got.Equal(time.Date(2026, 11, 8, 0, 5, 0, 0, time.UTC)) {
		t.Fatalf("expires = %v", got)
	}
	if len(st.objects) != 0 {
		t.Fatal("CreateUpload wrote an object")
	}
}

func TestCreateUploadRejectsLikeUploadFile(t *testing.T) {
	st := newMemoryStorage()
	svc := newTestUploadService(st)
	for _, req := range []*adminv1.CreateUploadRequest{
		{Filename: "a.png", Size: 0},
		{Filename: "a.png", Size: 101},
		{Filename: "a.exe", Size: 1},
		{Filename: "a.svg", Size: 1},
	} {
		_, err := svc.CreateUpload(context.Background(), req)
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
			t.Errorf("%+v: err = %v, want InvalidArgument", req, err)
		}
	}
	if len(st.presigns) != 0 {
		t.Fatalf("rejected requests were presigned: %v", st.presigns)
	}
	st.fail = true
	_, err := svc.CreateUpload(context.Background(), &adminv1.CreateUploadRequest{Filename: "a.png", Size: 1})
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInternal {
		t.Fatalf("err = %v, want Internal", err)
	}
}

// asConnectErr 解包 connect.Error。
func asConnectErr(err error) (*connect.Error, bool) {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}
