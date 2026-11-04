package service

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// normalizePageStrict 在 service 层归一化分页参数。
// page_size < 0 返回 InvalidArgument；page_size == 0 取默认值；> MaxPageSize 截断为 MaxPageSize。
// page_num < 1 归一化为 1。
// repo.normalizePage 保留为防御兜底。
func normalizePageStrict(pageNum, pageSize int) (int, int, error) {
	if pageSize < 0 {
		return 0, 0, connect.Errorf(connect.CodeInvalidArgument, "page_size 不能为负数")
	}
	if pageSize == 0 {
		pageSize = repo.DefaultPageSize
	}
	if pageSize > repo.MaxPageSize {
		pageSize = repo.MaxPageSize
	}
	if pageNum < 1 {
		pageNum = 1
	}
	return pageNum, pageSize, nil
}

// 统一的 service 层输入校验工具：必填、长度、枚举、格式。
// 校验失败统一返回 connect.CodeInvalidArgument，便于前端稳定提示。

var (
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	// phoneRe follows the current User product policy: mainland China mobile
	// numbers only, without separators or an international country prefix.
	phoneRe = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
)

// validator 收集字段错误，最终汇总为一个 InvalidArgument 错误。
type validator struct{ errs []string }

func newValidator() *validator { return &validator{} }

// required 校验字符串非空（去除首尾空白后）。
func (v *validator) required(field, value string) *validator {
	if strings.TrimSpace(value) == "" {
		v.errs = append(v.errs, field+"不能为空")
	}
	return v
}

// maxLen 校验字符串长度（按 Unicode 字符计）不超过 max。
func (v *validator) maxLen(field, value string, max int) *validator {
	if utf8.RuneCountInString(value) > max {
		v.errs = append(v.errs, fmt.Sprintf("%s长度不能超过 %d 个字符", field, max))
	}
	return v
}

// email 校验邮箱格式（空值跳过，视为可选）。
// field 参数保持 API 与 required/maxLen 一致（链式校验器风格），实际调用点总是 "邮箱"。
//
//nolint:unparam // 封装一致性：与其他 validator 方法同构
func (v *validator) email(field, value string) *validator {
	if value != "" && !emailRe.MatchString(value) {
		v.errs = append(v.errs, field+"格式不正确")
	}
	return v
}

// phone 校验中国大陆手机号（空值跳过，视为可选）。
// 当前产品不接受国际号码、空格、短号或座机号码。
//
//nolint:unparam // 封装一致性：与其他 validator 方法同构
func (v *validator) phone(field, value string) *validator {
	if value != "" && !phoneRe.MatchString(value) {
		v.errs = append(v.errs, field+"格式不正确")
	}
	return v
}

// status 校验状态枚举：仅允许“启用 / 停用”。
func (v *validator) status(s adminv1.Status) *validator {
	if s != adminv1.Status_STATUS_ENABLED && s != adminv1.Status_STATUS_DISABLED {
		v.errs = append(v.errs, "状态值非法（应为启用或停用）")
	}
	return v
}

// err 返回汇总后的 connect 错误；无错误返回 nil。
func (v *validator) err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return connect.Errorf(connect.CodeInvalidArgument, "%s", strings.Join(v.errs, "；"))
}
