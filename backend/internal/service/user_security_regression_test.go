package service

import (
	"context"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func newUserSecurityHandler(t *testing.T, base *repo.Repo) adminv1connect.UserServiceHandler {
	t.Helper()
	handler, err := NewGeneratedUserService(base)
	if err != nil {
		t.Fatalf("NewGeneratedUserService: %v", err)
	}
	return handler
}

func userClaimsContext(userID int64, username string) context.Context {
	return interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: userID, Username: username})
}

func createSecurityUser(t *testing.T, handler adminv1connect.UserServiceHandler, ctx context.Context, username string, deptID int64, roleIDs []int64) *adminv1.User {
	t.Helper()
	response, err := handler.CreateUser(ctx, &adminv1.CreateUserRequest{
		Username: "u-" + username,
		Password: "GoodPass1",
		Nickname: username,
		Email:    username + "@example.com",
		DeptId:   deptID,
		RoleIds:  roleIDs,
		Remark:   "security regression",
	})
	if err != nil {
		t.Fatalf("CreateUser(%q): %v", username, err)
	}
	if response.User == nil || response.User.Id == 0 {
		t.Fatalf("CreateUser(%q) returned no User: %v", username, response.User)
	}
	return response.User
}

func roleIDsOfUser(t *testing.T, base *repo.Repo, userID int64) []int64 {
	t.Helper()
	ids, err := base.RoleIDsOf(context.Background(), userID)
	if err != nil {
		t.Fatalf("load User role IDs: %v", err)
	}
	return ids
}

func containsUserID(users []*adminv1.User, id int64) bool {
	for _, user := range users {
		if user.Id == id {
			return true
		}
	}
	return false
}
