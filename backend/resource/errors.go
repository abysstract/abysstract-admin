package resource

import (
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicate 唯一键冲突（如用户名、角色标识、参数键、字典类型重复）。
var ErrDuplicate = errors.New("记录已存在：唯一字段冲突")

// ErrCycle 树形结构出现环：父节点不能是自身或自身的后代。
var ErrCycle = errors.New("非法的父节点：不能选择自身或其子节点")

// ErrOutOfRange 数值超出数据库列的取值范围，例如向 MySQL/MariaDB UNSIGNED 列写入负数。
var ErrOutOfRange = errors.New("数值超出字段取值范围")

// ErrForeignKey 外键约束拒绝写入：引用的记录不存在，或记录仍被其他数据引用。
var ErrForeignKey = errors.New("关联数据不存在或仍被引用")

// ErrCheckViolation 写入的值不满足数据库 CHECK 约束。
var ErrCheckViolation = errors.New("字段取值不满足数据约束")

// ErrInternal is the client message of unexpected failures; the cause is logged.
var ErrInternal = errors.New("服务内部错误")

// RejectedError 是数据库按业务规则主动拒绝的写入，例如触发器中的
// MySQL/MariaDB SIGNAL SQLSTATE '45000' 或 PostgreSQL RAISE EXCEPTION。
// Message 是应用在数据库中写给用户的提示（MESSAGE_TEXT），不含驱动信息。
type RejectedError struct {
	Message string
	Err     error
}

func (e *RejectedError) Error() string { return e.Message }
func (e *RejectedError) Unwrap() error { return e.Err }

// PostgreSQL SQLSTATE。
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
	pgRaiseException      = "P0001"
)

// MySQL/MariaDB 错误号。
const (
	// mysqlOutOfRange ER_WARN_DATA_OUT_OF_RANGE，严格模式下拒绝越界写入。
	mysqlOutOfRange = 1264
	// mysqlRowIsReferenced ER_ROW_IS_REFERENCED_2：删除或修改仍被引用的记录。
	mysqlRowIsReferenced = 1451
	// mysqlNoReferencedRow ER_NO_REFERENCED_ROW_2：引用的记录不存在。
	mysqlNoReferencedRow = 1452
	// mysqlCheckViolation ER_CHECK_CONSTRAINT_VIOLATED。
	mysqlCheckViolation = 3819
)

// NormalizeDBError 将底层数据库错误归一化为 资源层可识别错误。
// 唯一约束冲突映射为 ErrDuplicate；MySQL/MariaDB 越界写入映射为 ErrOutOfRange；
// 外键与 CHECK 约束映射为 ErrForeignKey、ErrCheckViolation；触发器等以用户
// 自定义 SQLSTATE（MySQL/MariaDB 45xxx、PostgreSQL P0001）拒绝的写入映射为
// *RejectedError。便于 service 层转成明确的接口错误，且不向客户端暴露驱动信息。
func NormalizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicate
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return ErrDuplicate
		case pgForeignKeyViolation:
			return fmt.Errorf("%w: %w", ErrForeignKey, err)
		case pgCheckViolation:
			return fmt.Errorf("%w: %w", ErrCheckViolation, err)
		case pgRaiseException:
			return &RejectedError{Message: pgErr.Message, Err: err}
		}
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) {
		switch {
		case mysqlErr.Number == mysqlOutOfRange:
			return ErrOutOfRange
		case mysqlErr.Number == mysqlRowIsReferenced || mysqlErr.Number == mysqlNoReferencedRow:
			return fmt.Errorf("%w: %w", ErrForeignKey, err)
		case mysqlErr.Number == mysqlCheckViolation:
			return fmt.Errorf("%w: %w", ErrCheckViolation, err)
		case mysqlErr.SQLState[0] == '4' && mysqlErr.SQLState[1] == '5':
			// SQLSTATE class 45 is reserved for user-defined conditions (SIGNAL).
			return &RejectedError{Message: mysqlErr.Message, Err: err}
		}
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
