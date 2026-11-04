package service_test

import (
	"context"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// createUserForTest 通过 repo.CreateUser 创建测试用户（自动 bcrypt + 设 create_by=0）。
// 上下文不带 Claims，create_by 退化为 0（符合历史/系统创建语义）。
func createUserForTest(r *repo.Repo, username, password string) (*model.SysUser, error) {
	u := &model.SysUser{
		DeptID:   new(int64(1)),
		Username: username,
		Nickname: username,
		Status:   1,
	}
	if err := r.CreateUser(context.Background(), u, password, []int64{}); err != nil {
		return nil, err
	}
	return u, nil
}
