package resource

import (
	"errors"

	"gorm.io/gorm"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicate 唯一键冲突（如用户名、角色标识、参数键、字典类型重复）。
var ErrDuplicate = errors.New("记录已存在：唯一字段冲突")

// ErrCycle 树形结构出现环：父节点不能是自身或自身的后代。
var ErrCycle = errors.New("非法的父节点：不能选择自身或其子节点")

// pgUniqueViolation PostgreSQL 唯一约束冲突的 SQLSTATE。
const pgUniqueViolation = "23505"

// NormalizeDBError 将底层数据库错误归一化为 资源层可识别错误。
// 当前主要把唯一约束冲突映射为 ErrDuplicate，便于 service 层转成明确的接口错误。
func NormalizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicate
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrDuplicate
	}
	return err
}

// ErrNotFound is shared with GORM lookups and zero-row resource writes.
var ErrNotFound = gorm.ErrRecordNotFound

// ErrEmptyPatch reports a selective update with zero columns.
var ErrEmptyPatch = errors.New("empty patch: no columns to update")

// ErrInvalidUpdateColumns reports unknown or duplicate update columns.
var ErrInvalidUpdateColumns = errors.New("invalid selective update columns")

// ErrHasChildren reports that a tree node still owns children.
var ErrHasChildren = errors.New("存在子节点，不能删除")
