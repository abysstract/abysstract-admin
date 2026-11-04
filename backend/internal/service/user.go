package service

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

// currentUserID 从 context 取出当前登录用户 ID；未登录返回 0。
func currentUserID(ctx context.Context) int64 {
	if c, ok := interceptor.ClaimsFromContext(ctx); ok {
		return c.UserID
	}
	return 0
}

// UserService 承载 User 的 typed CUSTOM 安全 workflow。标准 List 由 generated UserController 提供。
type UserService struct {
	repo          *repo.Repo
	refresh       *repo.RefreshTokenRepo // 可为 nil（未启用刷新令牌时）
	refreshEnable bool
}

// NewUserService 创建 UserService。默认不吊销 refresh token；
// 启用刷新令牌时通过 WithRefreshTokenRepo 注入 refresh repo。
func NewUserService(r *repo.Repo) *UserService {
	return &UserService{repo: r}
}

// WithRefreshTokenRepo 注入 RefreshTokenRepo 并启用重置密码时的 refresh 吊销。
// 返回新实例避免 mutating 共享 receiver。
func (s *UserService) WithRefreshTokenRepo(r *repo.RefreshTokenRepo) *UserService {
	cp := *s
	cp.refresh = r
	cp.refreshEnable = true
	return &cp
}

func toProtoUser(u *model.SysUser, roleIDs []int64) *adminv1.User {
	pu := &adminv1.User{
		Id:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Phone:    u.Phone,
		Status:   adminv1.Status(int32(u.Status)),

		RoleIds:    roleIDs,
		Remark:     u.Remark,
		CreateTime: timestamppb.New(u.CreateTime),
		UpdateTime: timestamppb.New(u.UpdateTime),
	}
	if u.DeptID != nil {
		pu.DeptId = *u.DeptID
	}
	return pu
}

func (s *UserService) GetUser(
	ctx context.Context, req *adminv1.GetUserRequest,
) (*adminv1.GetUserResponse, error) {
	if _, hasClaims := interceptor.ClaimsFromContext(ctx); !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, "未认证")
	}
	u, err := s.repo.GetUserResource(ctx, req.Id, s.repo.DataScopeAccessScope(ctx, "dept_id", "create_by"))
	if err != nil {
		return nil, toConnectError(err)
	}
	roleIDs, err := s.repo.RoleIDsOf(ctx, u.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.GetUserResponse{User: toProtoUser(u, roleIDs)}, nil
}

func (s *UserService) CreateUser(
	ctx context.Context, req *adminv1.CreateUserRequest,
) (*adminv1.CreateUserResponse, error) {
	msg := req
	msg.Username = identity.ASCIICaseKey(msg.Username)
	// User's write RPCs intentionally remain typed CUSTOM workflows, so the
	// generated Resource validators are not called by a generated Controller.
	// Reuse them here to keep Schema MaxLen/Required changes effective on the
	// custom path as well; password/format policy remains workflow-specific.
	if err := resourcegen.UserValidateCreate(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	if v := newValidator().
		required("密码", msg.Password).
		maxLen("密码", msg.Password, 100).
		email("邮箱", msg.Email).
		phone("手机号", msg.Phone).
		err(); v != nil {
		return nil, v
	}
	u := &model.SysUser{
		Username: msg.Username,
		Nickname: msg.Nickname,
		Email:    msg.Email,
		Phone:    msg.Phone,
		DeptID:   new(msg.DeptId),
		Remark:   msg.Remark,
		Status:   int16(adminv1.Status_STATUS_ENABLED),
	}
	if err := s.repo.CreateUser(ctx, u, msg.Password, msg.RoleIds); err != nil {
		// User 的唯一业务字段是 username。不要把 repo 层用于多个资源的
		// 通用唯一冲突文案直接暴露给用户。
		if errors.Is(err, repo.ErrDuplicate) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errUsernameExists.Error()).WithCause(errUsernameExists)
		}
		return nil, toConnectError(err)
	}
	return &adminv1.CreateUserResponse{User: toProtoUser(u, msg.RoleIds)}, nil
}

func (s *UserService) UpdateUser(
	ctx context.Context, req *adminv1.UpdateUserRequest,
) (*adminv1.UpdateUserResponse, error) {
	msg := req
	// The built-in administrator is an owner-only security boundary. A user
	// with ordinary user:edit permission must not be able to replace its
	// profile or role bindings; the built-in administrator may still update
	// itself through this workflow, just as it may reset its own password.
	if msg.GetId() == builtinAdminID && currentUserID(ctx) != builtinAdminID {
		return nil, connect.NewError(connect.CodePermissionDenied, errProtectBuiltinAdminUpdate.Error()).WithCause(errProtectBuiltinAdminUpdate)
	}
	// Keep custom Update presence-aware while sourcing field length checks from
	// the same generated validator used by standard Resource controllers.
	if err := resourcegen.UserValidateUpdate(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	v := newValidator()
	if msg.Email != nil {
		v.email("邮箱", msg.GetEmail())
	}
	if msg.Phone != nil {
		v.phone("手机号", msg.GetPhone())
	}
	if err := v.err(); err != nil {
		return nil, err
	}

	// UpdateUser remains a dedicated workflow in the User partial Resource.
	// Read the current row first so optional profile fields never turn into
	// accidental zero-value/password writes; role_ids remains owned by this
	// workflow and follows its existing full-replacement semantics.
	current, err := s.repo.GetUser(ctx, msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}
	u := *current
	if msg.Nickname != nil {
		u.Nickname = msg.GetNickname()
	}
	if msg.Email != nil {
		u.Email = msg.GetEmail()
	}
	if msg.Phone != nil {
		u.Phone = msg.GetPhone()
	}
	if msg.DeptId != nil {
		u.DeptID = new(msg.GetDeptId())
	}
	if msg.Remark != nil {
		u.Remark = msg.GetRemark()
	}
	roleIDs := msg.RoleIds
	if err := s.repo.UpdateUser(ctx, &u, roleIDs); err != nil {
		return nil, toConnectError(err)
	}
	updated, err := s.repo.GetUser(ctx, msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.UpdateUserResponse{User: toProtoUser(updated, roleIDs)}, nil
}

func (s *UserService) DeleteUser(
	ctx context.Context, req *adminv1.DeleteUserRequest,
) (*adminv1.DeleteUserResponse, error) {
	id := req.Id
	if id == builtinAdminID {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectBuiltinAdminDelete.Error()).WithCause(errProtectBuiltinAdminDelete)
	}
	if id == currentUserID(ctx) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectSelfDelete.Error()).WithCause(errProtectSelfDelete)
	}
	if err := s.repo.DeleteUser(ctx, id); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.DeleteUserResponse{}, nil
}

// BatchDeleteUsers is a typed CUSTOM workflow rather than a CommonExport
// resource dispatch. The repository validates the complete active ID set
// inside one transaction, so a missing/protected ID cannot cause partial
// deletion.
func (s *UserService) BatchDeleteUsers(
	ctx context.Context, req *adminv1.BatchDeleteUsersRequest,
) (*adminv1.BatchDeleteUsersResponse, error) {
	if len(req.Ids) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errBatchDeleteEmpty.Error()).WithCause(errBatchDeleteEmpty)
	}
	if len(req.Ids) > maxBatchDelete {
		return nil, connect.NewError(connect.CodeInvalidArgument, errBatchDeleteTooMany.Error()).WithCause(errBatchDeleteTooMany)
	}
	// Return the same stable protection errors as single-user DeleteUser.
	// The repository repeats this check as defense in depth, but its sentinel
	// is intentionally private to repo and must not leak as an Internal error
	// through the typed RPC.
	for _, id := range req.Ids {
		if id == builtinAdminID {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectBuiltinAdminDelete.Error()).WithCause(errProtectBuiltinAdminDelete)
		}
		if id == currentUserID(ctx) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectSelfDelete.Error()).WithCause(errProtectSelfDelete)
		}
	}
	deleted, err := s.repo.BatchDeleteUsers(ctx, req.Ids)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.BatchDeleteUsersResponse{Deleted: int32(deleted)}, nil
}

func (s *UserService) ChangeUserStatus(
	ctx context.Context, req *adminv1.ChangeUserStatusRequest,
) (*adminv1.ChangeUserStatusResponse, error) {
	id := req.Id
	if v := newValidator().status(req.Status).err(); v != nil {
		return nil, v
	}
	// 仅在“停用”动作时保护内置管理员与当前登录用户。
	if req.Status == adminv1.Status_STATUS_DISABLED {
		if id == builtinAdminID {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectBuiltinAdminDisable.Error()).WithCause(errProtectBuiltinAdminDisable)
		}
		if id == currentUserID(ctx) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errProtectSelfDisable.Error()).WithCause(errProtectSelfDisable)
		}
	}
	if err := s.repo.ChangeStatus(ctx, id, int16(req.Status)); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.ChangeUserStatusResponse{}, nil
}

func (s *UserService) ResetPassword(
	ctx context.Context, req *adminv1.ResetPasswordRequest,
) (*adminv1.ResetPasswordResponse, error) {
	if v := newValidator().
		required("新密码", req.NewPassword).
		maxLen("新密码", req.NewPassword, 100).
		err(); v != nil {
		return nil, v
	}
	// 重置内置管理员密码仅限其本人操作，避免被其他管理员重置后接管。
	if req.Id == builtinAdminID && currentUserID(ctx) != builtinAdminID {
		return nil, connect.NewError(connect.CodePermissionDenied, errProtectBuiltinAdminPwd.Error()).WithCause(errProtectBuiltinAdminPwd)
	}
	// 密码重置、强制改密标记、会话版本和刷新令牌状态在同一事务内更新。
	var refresh *repo.RefreshTokenRepo
	if s.refreshEnable {
		refresh = s.refresh
	}
	if err := s.repo.ResetPasswordAndRevokeRefresh(ctx, req.Id, req.NewPassword, refresh); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.ResetPasswordResponse{}, nil
}

func toConnectError(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	}
	if errors.Is(err, repo.ErrProtectedUserUpdate) {
		return connect.NewError(connect.CodePermissionDenied, errProtectBuiltinAdminUpdate.Error()).WithCause(errProtectBuiltinAdminUpdate)
	}
	if errors.Is(err, repo.ErrDuplicate) {
		return connect.NewError(connect.CodeAlreadyExists, err.Error()).WithCause(err)
	}
	if errors.Is(err, repo.ErrHasChildren) {
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	}
	if errors.Is(err, repo.ErrLastActiveRole) || errors.Is(err, repo.ErrProtectedRole) {
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	}
	if errors.Is(err, repo.ErrCycle) {
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
}
