package service

import (
	"context"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// ProfileService 实现 adminv1connect.ProfileServiceHandler（需求 9.13）。
// 当前用户读写自己的资料：username 只读；nickname/email/phone 可改；avatar 由 UploadAvatar 维护。
type ProfileService struct {
	repo *repo.Repo
}

func NewProfileService(r *repo.Repo) *ProfileService {
	return &ProfileService{repo: r}
}

func (s *ProfileService) GetProfile(
	ctx context.Context, _ *adminv1.GetProfileRequest,
) (*adminv1.GetProfileResponse, error) {
	uid := currentUserID(ctx)
	if uid == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	u, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.GetProfileResponse{
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
	}, nil
}

func (s *ProfileService) UpdateProfile(
	ctx context.Context, req *adminv1.UpdateProfileRequest,
) (*adminv1.UpdateProfileResponse, error) {
	msg := req
	uid := currentUserID(ctx)
	if uid == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errNoClaims.Error()).WithCause(errNoClaims)
	}
	if v := newValidator().
		maxLen("昵称", msg.Nickname, 64).
		email("邮箱", msg.Email).
		maxLen("手机号", msg.Phone, 32).
		err(); v != nil {
		return nil, v
	}
	// 复用 repo.UpdateUser，但只更新资料字段；roleIDs/dept 传零值表示不动。
	// 注意：UpdateUser 会写入 dept_id=0；为避免误改，使用 UpdateProfileOwn 专用 repo 方法更稳。
	if err := s.repo.UpdateProfileOwn(ctx, uid, msg.Nickname, msg.Email, msg.Phone); err != nil {
		return nil, toConnectError(err)
	}
	u, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.UpdateProfileResponse{
		Profile: &adminv1.GetProfileResponse{
			Username: u.Username,
			Nickname: u.Nickname,
			Email:    u.Email,
			Phone:    u.Phone,
			Avatar:   u.Avatar,
		},
	}, nil
}
