// Package testdb admits explicitly configured disposable databases
// for destructive integration tests. Application configuration stays separate.
package testdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
)

const (
	DSNEnv      = "ADMIN_TEST_DSN"
	DatabaseEnv = "ADMIN_TEST_DATABASE"
	RunIDEnv    = "ADMIN_TEST_RUN_ID"
)

// Config binds the exact exported DSN to a disposable database and its run.
type Config struct{ Driver, DSN, Database, RunID string }

var runPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// FromEnv reads only the dedicated test configuration.
func FromEnv() (Config, error) {
	driver := os.Getenv("ADMIN_TEST_DRIVER")
	if driver == "" {
		driver = "postgres"
	}
	c := Config{Driver: driver, DSN: os.Getenv(DSNEnv), Database: os.Getenv(DatabaseEnv), RunID: os.Getenv(RunIDEnv)}
	return c, c.Validate()
}

// DatabaseName identifies one run's disposable database.
func DatabaseName(runID string) (string, error) {
	if !runPattern.MatchString(runID) {
		return "", errors.New("testdb: run ID must contain 32 lowercase hexadecimal characters")
	}
	return "admin_core_k2_" + runID, nil
}

// Marker is the PostgreSQL database comment written at provisioning time.
func (c Config) Marker() string { return "abysstract-admin:test:" + c.RunID }

// Validate checks the actual exported string before opening any connection.
func (c Config) Validate() error {
	if c.Driver == "sqlite" {
		return c.validateSQLite()
	}
	if c.Driver != "" && c.Driver != "postgres" && c.Driver != "mysql" && c.Driver != "mariadb" {
		return errors.New("testdb: ADMIN_TEST_DRIVER must be postgres, mysql, mariadb or sqlite")
	}
	name, err := DatabaseName(c.RunID)
	if err != nil {
		return err
	}
	if c.Database != name {
		return errors.New("testdb: expected database must match the run ID")
	}
	if c.DSN == "" {
		return errors.New("testdb: ADMIN_TEST_DSN is required")
	}
	if c.Driver == "mysql" || c.Driver == "mariadb" {
		parsed, err := mysqldriver.ParseDSN(c.DSN)
		if err != nil {
			return errors.New("testdb: invalid test DSN")
		}
		if parsed.DBName != c.Database {
			return errors.New("testdb: exported DSN targets a different database")
		}
		return nil
	}
	parsed, err := pgx.ParseConfig(c.DSN)
	if err != nil {
		return errors.New("testdb: invalid test DSN")
	}
	if parsed.Database != c.Database {
		return errors.New("testdb: exported DSN targets a different database")
	}
	return nil
}

// ForDatabase constructs a new DSN while preserving connection options. The
// result is parsed again; it is the sole string used by verification and tests.
func ForDatabase(base, database, runID string) (Config, error) {
	c := Config{Driver: "postgres", Database: database, RunID: runID}
	if base == "" {
		return c, errors.New("testdb: explicit base DSN is required")
	}
	u, err := url.Parse(base)
	if err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		u.Path = "/" + database
		u.RawPath = ""
		q := u.Query()
		q.Del("dbname")
		u.RawQuery = q.Encode()
		c.DSN = u.String()
	} else {
		// DatabaseName validation below limits identifiers to a fixed safe alphabet.
		c.DSN = base + " dbname='" + database + "'"
	}
	return c, c.Validate()
}

// Verify validates both the supplied configuration and the actual connection.
// PostgreSQL uses a database comment; MySQL uses the retained p0_owner row.
func Verify(ctx context.Context, db *gorm.DB, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if db == nil {
		return errors.New("testdb: database handle is required")
	}
	if c.Driver == "sqlite" {
		return verifySQLite(ctx, db, c)
	}
	if c.Driver == "mysql" || c.Driver == "mariadb" {
		var actual string
		var ownership struct {
			Count int64
			RunID string
		}
		if err := db.WithContext(ctx).Raw("SELECT DATABASE()").Scan(&actual).Error; err != nil {
			return errors.New("testdb: database identity query failed")
		}
		if actual != c.Database {
			return errors.New("testdb: connected database differs from the authorized target")
		}
		if err := db.WithContext(ctx).Raw("SELECT COUNT(*) AS count, COALESCE(MAX(run_id), '') AS run_id FROM p0_owner").Scan(&ownership).Error; err != nil {
			return errors.New("testdb: ownership query failed")
		}
		if ownership.Count != 1 || ownership.RunID != c.RunID {
			return errors.New("testdb: database ownership marker differs from this run")
		}
		return nil
	}
	var identity struct{ Database, Marker string }
	err := db.WithContext(ctx).Raw(`SELECT current_database() AS database,
 COALESCE(shobj_description(oid, 'pg_database'), '') AS marker
 FROM pg_database WHERE datname = current_database()`).Scan(&identity).Error
	if err != nil {
		return errors.New("testdb: database identity query failed")
	}
	return c.verifyIdentity(identity.Database, identity.Marker)
}

func (c Config) verifyIdentity(database, marker string) error {
	if database != c.Database {
		return errors.New("testdb: connected database differs from the authorized target")
	}
	if marker != c.Marker() {
		return errors.New("testdb: database ownership marker differs from this run")
	}
	return nil
}

// Open returns a handle only after admission. Failures close the newly opened
// pool. Connection errors are sanitized to keep DSN secrets out of test logs.
func Open(c Config) (*gorm.DB, error) {
	return open(c, func(dsn string) (*gorm.DB, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		driver := c.Driver
		if driver == "" {
			driver = "postgres"
		}
		return database.Open(ctx, driver, dsn)
	}, func(db *gorm.DB) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return Verify(ctx, db, c)
	})
}

func open(c Config, dial func(string) (*gorm.DB, error), verify func(*gorm.DB) error) (*gorm.DB, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	db, err := dial(c.DSN)
	if err != nil {
		return nil, errors.New("testdb: open test database failed")
	}
	if err := verify(db); err != nil {
		if pool, poolErr := db.DB(); poolErr == nil {
			if closeErr := pool.Close(); closeErr != nil {
				return nil, fmt.Errorf("testdb: admission rejected and pool cleanup failed: %w", err)
			}
		}
		return nil, err
	}
	return db, nil
}
