package resource

import (
	"errors"
	"fmt"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeDBErrorMapsOutOfRange(t *testing.T) {
	outOfRange := fmt.Errorf("insert: %w", &mysqldriver.MySQLError{Number: 1264, Message: "Out of range value for column 'set_id' at row 1"})
	if got := NormalizeDBError(outOfRange); !errors.Is(got, ErrOutOfRange) {
		t.Fatalf("MySQL 1264 = %v, want ErrOutOfRange", got)
	}
	other := &mysqldriver.MySQLError{Number: 1366}
	if got := NormalizeDBError(other); got != other {
		t.Fatalf("unrelated MySQL error changed: %v", got)
	}
	// PostgreSQL numeric_value_out_of_range keeps its existing behavior.
	pg := &pgconn.PgError{Code: "22003"}
	if got := NormalizeDBError(pg); got != pg {
		t.Fatalf("PostgreSQL error changed: %v", got)
	}
}

func TestNormalizeDBErrorMapsConstraintsAndRejections(t *testing.T) {
	signal := &mysqldriver.MySQLError{Number: 1644, SQLState: [5]byte{'4', '5', '0', '0', '0'}, Message: "已发布的笔记不能修改"}
	raise := &pgconn.PgError{Code: "P0001", Message: "已发布的笔记不能修改"}
	for _, err := range []error{signal, fmt.Errorf("update: %w", raise)} {
		var rejected *RejectedError
		if got := NormalizeDBError(err); !errors.As(got, &rejected) || rejected.Message != "已发布的笔记不能修改" || got.Error() != rejected.Message {
			t.Fatalf("%v = %#v, want RejectedError with the database message", err, got)
		}
	}
	for _, err := range []error{
		&mysqldriver.MySQLError{Number: 1452, SQLState: [5]byte{'2', '3', '0', '0', '0'}, Message: "Cannot add or update a child row: a foreign key constraint fails (`db`.`t`, CONSTRAINT ...)"},
		&mysqldriver.MySQLError{Number: 1451, SQLState: [5]byte{'2', '3', '0', '0', '0'}},
		&pgconn.PgError{Code: "23503"},
	} {
		got := NormalizeDBError(err)
		if !errors.Is(got, ErrForeignKey) || !errors.Is(got, err) {
			t.Fatalf("%v = %v, want ErrForeignKey wrapping the cause", err, got)
		}
	}
	for _, err := range []error{&mysqldriver.MySQLError{Number: 3819}, &pgconn.PgError{Code: "23514"}} {
		if got := NormalizeDBError(err); !errors.Is(got, ErrCheckViolation) {
			t.Fatalf("%v = %v, want ErrCheckViolation", err, got)
		}
	}
	// Other integrity errors keep their cause for the generic internal path.
	notNull := &mysqldriver.MySQLError{Number: 1048, SQLState: [5]byte{'2', '3', '0', '0', '0'}}
	if got := NormalizeDBError(notNull); got != notNull {
		t.Fatalf("unrelated MySQL error changed: %v", got)
	}
}
