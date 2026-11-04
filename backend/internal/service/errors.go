package service

import "errors"

// builtinAdminID 内置超级管理员用户 ID（见 migrate.seedAdmin）。
const builtinAdminID int64 = 1

var (
	errAccountDisabled = errors.New("账号已停用")
	errNoClaims        = errors.New("未获取到登录信息")

	errProtectBuiltinAdminDelete  = errors.New("内置管理员不可删除")
	errProtectBuiltinAdminDisable = errors.New("内置管理员不可停用")
	errProtectSelfDelete          = errors.New("不可删除当前登录用户")
	errProtectSelfDisable         = errors.New("不可停用当前登录用户")
	errProtectBuiltinAdminPwd     = errors.New("只有内置管理员本人可以重置其密码")
	errProtectBuiltinAdminUpdate  = errors.New("只有内置管理员本人可以修改内置管理员")
	errUsernameExists             = errors.New("用户名已存在，请更换用户名")
)
