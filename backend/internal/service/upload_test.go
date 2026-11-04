package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/quick"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

// memorySaver 记录所有 Save 调用，便于断言「校验失败时不调用 Save」。
type memorySaver struct {
	saved  []string
	urlFor func(string) string
	fail   bool
}

func (m *memorySaver) Save(_ context.Context, filename string, _ []byte) (string, error) {
	if m.fail {
		return "", errors.New("saver injected failure")
	}
	m.saved = append(m.saved, filename)
	if m.urlFor != nil {
		return m.urlFor(filename), nil
	}
	return "/static/" + filename, nil
}

// 属性 13：上传校验
// 对任意（大小、扩展）组合，接受 ⇔ 大小 ≤ 上限且扩展在白名单内；
// 被拒绝时 Save 不被调用（即不保存），并返回描述性错误。
func TestPropertyUploadValidation(t *testing.T) {
	const maxBytes int64 = 100
	allowedExts := defaultUploadExts

	check := func(size int, ext string) bool {
		filename := "f" + ext
		err := validateUpload(filename, int64(size), maxBytes, allowedExts)
		accepted := err == nil
		sizeOK := size > 0 && int64(size) <= maxBytes
		// validateUpload 内部用 strings.ToLower，所以这里等价比较也用小写。
		extOK := allowedExts[strings.ToLower(ext)]
		// 接受当且仅当大小与扩展均通过
		if accepted != (sizeOK && extOK) {
			t.Errorf("size=%d ext=%q accepted=%v want=%v", size, ext, accepted, sizeOK && extOK)
			return false
		}
		// 被拒绝时返回的 error 必须非空（描述性错误）
		if !accepted && err == nil {
			t.Errorf("expected non-nil error for rejected upload")
			return false
		}
		return true
	}

	// 边界：size 覆盖 0/1/极限值/越界，扩展覆盖白名单 + 几个非白名单
	sizes := []int{0, 1, 50, 99, 100, 101, 1000}
	exts := []string{".png", ".pdf", ".zip", ".exe", ".html", "", ".PNG"}
	for _, sz := range sizes {
		for _, ex := range exts {
			if !check(sz, ex) {
				t.FailNow()
			}
		}
	}

	// 随机属性测试（快速 200 次）：扩展名从枚举集合随机取，避免随机字符串。
	rngExts := append([]string{}, ".png", ".pdf", ".zip", ".exe", ".html")
	if err := quick.Check(func(size int, extIdx byte) bool {
		ext := rngExts[int(extIdx)%len(rngExts)]
		sz := size % int(maxBytes+50)
		if sz < 0 {
			sz = -sz
		}
		return check(sz, ext)
	}, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("quick check failed: %v", err)
	}
}

// 属性 13 推论：UploadFile 在校验失败时不应调用 Saver.Save（不保存文件）。
// 头像场景同理（见 TestUploadAvatarDoesNotSaveOnValidationFailure）。
func TestUploadFileDoesNotSaveOnValidationFailure(t *testing.T) {
	saver := &memorySaver{}
	svc := NewUploadService(nil, saver, 100, 100)

	cases := []struct {
		name     string
		filename string
		content  []byte
	}{
		{"empty content", "a.png", nil},
		{"too large", "a.png", make([]byte, 101)},
		{"bad ext", "a.exe", []byte("x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UploadFile(context.Background(),
				&adminv1.UploadFileRequest{
					Filename: c.filename,
					Content:  c.content,
				})
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if connErr, ok := asConnectErr(err); ok && connErr.Code() != connect.CodeInvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", connErr.Code())
			}
			if len(saver.saved) != 0 {
				t.Errorf("saver should not be called on validation failure; got %d saves", len(saver.saved))
			}
		})
	}
}

// 属性 13 推论：通过校验时 saver 被调用，并返回引用 URL（size 同步透传）。
func TestUploadFileAcceptsAndSaves(t *testing.T) {
	saver := &memorySaver{}
	svc := NewUploadService(nil, saver, 100, 100)

	resp, err := svc.UploadFile(context.Background(),
		&adminv1.UploadFileRequest{
			Filename: "avatar.png",
			Content:  []byte("hello"),
		})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(saver.saved) != 1 {
		t.Fatalf("saver should be called exactly once; got %d", len(saver.saved))
	}
	if resp.Url == "" {
		t.Errorf("url should not be empty on success")
	}
	if resp.Size != 5 {
		t.Errorf("size mismatch: got %d want 5", resp.Size)
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
