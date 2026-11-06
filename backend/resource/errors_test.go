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
