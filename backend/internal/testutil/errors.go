package testutil

import (
	"errors"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
)

// RequireNativeError verifies that a non-duplicate error keeps its driver code.
func RequireNativeError(t *testing.T, db *gorm.DB, err error, postgresCode string, mysqlCode uint16) {
	t.Helper()
	if database.DriverOf(db) == "sqlite" {
		var native sqlite3.Error
		code := sqlite3.ErrConstraintForeignKey
		if postgresCode == "42P01" {
			if !errors.As(err, &native) || native.Code != sqlite3.ErrError {
				t.Fatalf("expected sqlite SQL error: %v", err)
			}
			return
		}
		if !errors.As(err, &native) || native.ExtendedCode != code {
			t.Fatalf("expected sqlite %d: %v", code, err)
		}
	} else if database.DriverOf(db) == "mysql" || database.DriverOf(db) == "mariadb" {
		var native *mysqldriver.MySQLError
		if !errors.As(err, &native) || native.Number != mysqlCode {
			t.Fatalf("expected mysql %d, got %v", mysqlCode, err)
		}
	} else {
		var native *pgconn.PgError
		if !errors.As(err, &native) || native.Code != postgresCode {
			t.Fatalf("expected postgres %s, got %v", postgresCode, err)
		}
	}
}
