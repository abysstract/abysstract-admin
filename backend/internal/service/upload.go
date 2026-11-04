package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// 文件 / 头像上传白名单与默认大小上限（需求 9.1–9.4）。
// 大小上限可通过 ADMIN_UPLOAD_MAX_BYTES / ADMIN_AVATAR_MAX_BYTES 覆盖（见 conf.go）。
var (
	defaultUploadExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".pdf": true, ".doc": true, ".docx": true,
		".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".txt": true, ".csv": true, ".zip": true,
	}
	defaultAvatarExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	}

	errUploadTooLarge      = errors.New("文件大小超过上限")
	errUploadEmpty         = errors.New("文件内容不能为空")
	errUploadExtNotAllowed = errors.New("文件类型不在允许的白名单内")
)

// Saver 抽象文件落盘逻辑，便于测试用内存 fake 替换。
// Save 成功返回可访问的 URL；失败返回错误（调用方将透传为 Internal）。
type Saver interface {
	Save(ctx context.Context, filename string, content []byte) (url string, err error)
}

// FileSystemSaver 把文件写入 dir，返回 "<urlPrefix><uuid-with-ext>"。
// 调用方（main.go）需在 mux 上挂载 urlPrefix 对应的 http.FileServer。
type FileSystemSaver struct {
	Dir       string
	URLPrefix string
}

func NewFileSystemSaver(dir, urlPrefix string) *FileSystemSaver {
	return &FileSystemSaver{Dir: dir, URLPrefix: urlPrefix}
}

func (s *FileSystemSaver) Save(_ context.Context, filename string, content []byte) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	var randBytes [16]byte
	if _, err := rand.Read(randBytes[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	name := hex.EncodeToString(randBytes[:]) + ext
	full := filepath.Join(s.Dir, name)
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return s.URLPrefix + name, nil
}

// validateUpload 纯函数：大小 + 扩展名白名单校验（需求 9.1/9.2/9.3/9.4）。
// 通过返回 nil；否则返回对应的描述性错误（调用方转 InvalidArgument）。
func validateUpload(filename string, size int64, maxBytes int64, allowedExts map[string]bool) error {
	if size <= 0 {
		return errUploadEmpty
	}
	if size > maxBytes {
		return fmt.Errorf("%w: %d > %d", errUploadTooLarge, size, maxBytes)
	}
	ext := strings.ToLower(path.Ext(filename))
	if !allowedExts[ext] {
		return fmt.Errorf("%w: %s", errUploadExtNotAllowed, ext)
	}
	return nil
}

// UploadService 实现 adminv1connect.UploadServiceHandler。
type UploadService struct {
	repo           *repo.Repo
	saver          Saver
	uploadMaxBytes int64
	avatarMaxBytes int64
	uploadExts     map[string]bool
	avatarExts     map[string]bool
}

func NewUploadService(r *repo.Repo, saver Saver, uploadMaxBytes, avatarMaxBytes int64) *UploadService {
	return &UploadService{
		repo:           r,
		saver:          saver,
		uploadMaxBytes: uploadMaxBytes,
		avatarMaxBytes: avatarMaxBytes,
		uploadExts:     defaultUploadExts,
		avatarExts:     defaultAvatarExts,
	}
}

// UploadFile 通用文件上传（需求 9.1/9.2）。
// 校验失败 → 返回 InvalidArgument 且不调用 saver（即不落盘）。
// 校验通过但 saver 失败 → 返回 Internal（属性 13 仅约束「拒绝→不保存」单向）。
func (s *UploadService) UploadFile(
	ctx context.Context, req *adminv1.UploadFileRequest,
) (*adminv1.UploadFileResponse, error) {
	msg := req
	if err := validateUpload(msg.Filename, int64(len(msg.Content)), s.uploadMaxBytes, s.uploadExts); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	url, err := s.saver.Save(ctx, msg.Filename, msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &adminv1.UploadFileResponse{
		Url:  url,
		Size: int64(len(msg.Content)),
	}, nil
}

// UploadAvatar 头像上传（需求 9.3/9.4）。
// 校验失败 → 返回 InvalidArgument、不更新 sys_user.avatar（保留原引用）。
// 校验通过 → 保存文件 + 更新 sys_user.avatar + 返回新引用。
// 若文件已保存但 DB 更新失败，仍返回 Internal；新引用仅对当前会话失败，可重试。
func (s *UploadService) UploadAvatar(
	ctx context.Context, req *adminv1.UploadAvatarRequest,
) (*adminv1.UploadAvatarResponse, error) {
	msg := req
	uid := currentUserID(ctx)
	if uid == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	if err := validateUpload(msg.Filename, int64(len(msg.Content)), s.avatarMaxBytes, s.avatarExts); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	url, err := s.saver.Save(ctx, msg.Filename, msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	if err := s.repo.UpdateAvatar(ctx, uid, url); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.UploadAvatarResponse{Url: url}, nil
}
