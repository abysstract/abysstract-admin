package coremigrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
)

// Run locks the target before constructing the provider. The caller owns the pool.
func Run(ctx context.Context, db *gorm.DB, sources fs.FS, selected ...*goose.Migration) (err error) {
	driver := database.DriverOf(db)
	if driver == "sqlite" {
		return runSQLite(ctx, db, sources, selected...)
	}
	if driver != "postgres" && driver != "mysql" && driver != "mariadb" {
		return fmt.Errorf("unsupported migration product")
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close()) }()
	var lockName string
	dialect := goose.DialectPostgres
	if driver == "mysql" || driver == "mariadb" {
		dialect = goose.DialectMySQL
		var target string
		if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&target); err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(target))
		lockName = fmt.Sprintf("abysstract-admin:%x", sum[:20])
	}
	if err := acquireMigrationLock(ctx, conn, driver, lockName); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released bool
		var releaseErr error
		if driver == "mysql" || driver == "mariadb" {
			releaseErr = conn.QueryRowContext(cleanup, "SELECT RELEASE_LOCK(?)", lockName).Scan(&released)
		} else {
			releaseErr = conn.QueryRowContext(cleanup, "SELECT pg_advisory_unlock($1)", int64(728195301)).Scan(&released)
		}
		if releaseErr == nil && !released {
			releaseErr = fmt.Errorf("migration lock release failed")
		}
		err = errors.Join(err, releaseErr)
	}()
	provider, err := goose.NewProvider(dialect, pool, sources,
		goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(selected...))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func acquireMigrationLock(ctx context.Context, conn *sql.Conn, driver, lockName string) error {
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		var acquired bool
		var lockErr error
		if driver == "mysql" || driver == "mariadb" {
			lockErr = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&acquired)
		} else {
			lockErr = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", int64(728195301)).Scan(&acquired)
		}
		if lockErr != nil {
			return lockErr
		}
		if acquired {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("migration lock timeout 300ms")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return nil
}
