package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/storage"
)

// 上传对象键前缀。键形如 files/2026/11/<32 位随机 hex>.png。
const (
	uploadKeyPrefix = "files"
	avatarKeyPrefix = "avatars"
)

// UploadConfig 是上传服务的限制项（需求 9.1–9.4）。
// 大小上限由 ADMIN_UPLOAD_MAX_BYTES / ADMIN_AVATAR_MAX_BYTES 配置。
type UploadConfig struct {
	FileMaxBytes   int64
	AvatarMaxBytes int64
	// UploadTTL 是 CreateUpload 预签名地址的有效期；0 使用 storage.DefaultUploadTTL。
	UploadTTL time.Duration
	// Now 仅供测试替换时钟（影响对象键中的年月）。
	Now func() time.Time
}

// UploadService 实现 adminv1connect.UploadServiceHandler，文件写入 storage.Storage。
type UploadService struct {
	repo         *repo.Repo
	store        storage.Storage
	filePolicy   storage.UploadPolicy
	avatarPolicy storage.UploadPolicy
	ttl          time.Duration
	now          func() time.Time
}

func NewUploadService(r *repo.Repo, st storage.Storage, cfg UploadConfig) *UploadService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &UploadService{
		repo:         r,
		store:        st,
		filePolicy:   storage.UploadPolicy{MaxBytes: cfg.FileMaxBytes, AllowedExts: storage.FileExts()},
		avatarPolicy: storage.UploadPolicy{MaxBytes: cfg.AvatarMaxBytes, AllowedExts: storage.ImageExts()},
		ttl:          cfg.UploadTTL,
		now:          now,
	}
}

// newKey 生成 <prefix>/<yyyy>/<mm>/<random><ext>。
func (s *UploadService) newKey(prefix, filename string) (string, error) {
	t := s.now().UTC()
	return storage.RandomKey(fmt.Sprintf("%s/%04d/%02d", prefix, t.Year(), int(t.Month())), filename)
}

// policyError 把存储层的校验错误转为带中文描述的 InvalidArgument。
func policyError(err error) error {
	msg := err.Error()
	switch {
	case errors.Is(err, storage.ErrEmpty):
		msg = "文件内容不能为空"
	case errors.Is(err, storage.ErrTooLarge):
		msg = "文件大小超过上限: " + err.Error()
	case errors.Is(err, storage.ErrExtNotAllowed):
		msg = "文件类型不在允许的白名单内: " + err.Error()
	}
	return connect.NewError(connect.CodeInvalidArgument, msg).WithCause(err)
}

func internalError(err error) error {
	return connect.NewError(connect.CodeInternal, "文件存储失败").WithCause(err)
}

// put 校验并写入存储，返回对象键。校验失败不写入（属性 13）。
func (s *UploadService) put(ctx context.Context, prefix, filename string, content []byte, policy storage.UploadPolicy) (string, error) {
	if err := policy.Check(filename, int64(len(content))); err != nil {
		return "", policyError(err)
	}
	key, err := s.newKey(prefix, filename)
	if err != nil {
		return "", internalError(err)
	}
	if err := s.store.Put(ctx, key, bytes.NewReader(content), int64(len(content)), storage.ContentTypeByExt(filename)); err != nil {
		return "", internalError(err)
	}
	return key, nil
}

// UploadFile 通用文件上传（需求 9.1/9.2）。
// 校验失败 → InvalidArgument 且不写入存储；写入失败 → Internal。
func (s *UploadService) UploadFile(
	ctx context.Context, req *adminv1.UploadFileRequest,
) (*adminv1.UploadFileResponse, error) {
	key, err := s.put(ctx, uploadKeyPrefix, req.Filename, req.Content, s.filePolicy)
	if err != nil {
		return nil, err
	}
	return &adminv1.UploadFileResponse{
		Url:  s.store.URL(key),
		Size: int64(len(req.Content)),
		Key:  key,
	}, nil
}

// UploadAvatar 头像上传（需求 9.3/9.4）。
// 校验失败 → InvalidArgument、不更新 sys_user.avatar（保留原引用）。
// 校验通过 → 写入存储 + 把 URL 写入 sys_user.avatar + 返回新引用。
// 若文件已保存但 DB 更新失败，仍返回 Internal；可重试。
func (s *UploadService) UploadAvatar(
	ctx context.Context, req *adminv1.UploadAvatarRequest,
) (*adminv1.UploadAvatarResponse, error) {
	uid := currentUserID(ctx)
	if uid == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	key, err := s.put(ctx, avatarKeyPrefix, req.Filename, req.Content, s.avatarPolicy)
	if err != nil {
		return nil, err
	}
	url := s.store.URL(key)
	if err := s.repo.UpdateAvatar(ctx, uid, url); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.UploadAvatarResponse{Url: url, Key: key}, nil
}

// CreateUpload 返回预签名直传地址，规则与 UploadFile 相同。
// 客户端按响应把文件 PUT 到 upload_url，再把 key 提交给业务接口；
// 业务接口应先用 storage.VerifyUpload 确认对象已上传。
func (s *UploadService) CreateUpload(
	ctx context.Context, req *adminv1.CreateUploadRequest,
) (*adminv1.CreateUploadResponse, error) {
	if err := s.filePolicy.Check(req.Filename, req.Size); err != nil {
		return nil, policyError(err)
	}
	key, err := s.newKey(uploadKeyPrefix, req.Filename)
	if err != nil {
		return nil, internalError(err)
	}
	res, err := storage.Presign(ctx, s.store, s.filePolicy, storage.PresignRequest{
		Key: key, Filename: req.Filename, Size: req.Size, TTL: s.ttl,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, "生成上传地址失败").WithCause(err)
	}
	return &adminv1.CreateUploadResponse{
		Key:       res.Key,
		Url:       res.URL,
		UploadUrl: res.Upload.URL,
		Method:    res.Upload.Method,
		Headers:   res.Upload.Headers,
		ExpiresAt: timestamppb.New(res.Upload.ExpiresAt),
	}, nil
}
