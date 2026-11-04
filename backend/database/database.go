// Package database owns database product identity and application connections.
package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type mysqlProduct struct {
	*mysql.Dialector
	product string
}

func (d mysqlProduct) Product() string { return d.product }
func (mysqlProduct) Translate(err error) error {
	var native *mysqldriver.MySQLError
	if errors.As(err, &native) && native.Number == 1062 {
		return gorm.ErrDuplicatedKey
	}
	return err
}

type postgresProduct struct{ *postgres.Dialector }

func (postgresProduct) Product() string { return "postgres" }

// Translate exposes the shared duplicate sentinel and preserves other native errors.
func (postgresProduct) Translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return gorm.ErrDuplicatedKey
	}
	return err
}

// Open selects a supported product and checks the connected server identity.
// The caller owns and closes the returned connection pool.
func Open(ctx context.Context, driver, dsn string) (*gorm.DB, error) {
	dialector, err := productDialector(driver, dsn)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		TranslateError: true,
		NowFunc:        func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	var version string
	switch driver {
	case "sqlite":
		err = db.WithContext(ctx).Raw("SELECT sqlite_version()").Scan(&version).Error
		if err == nil {
			err = checkMinimumVersion(driver, version, [3]int{3, 35, 0})
		}
		if err == nil {
			err = ApplyPool(db, Config{MaxOpenConns: 4, MaxIdleConns: 4})
		}
	case "postgres":
		err = db.WithContext(ctx).Raw("SHOW server_version").Scan(&version).Error
		if err == nil {
			err = checkMinimumVersion(driver, version, [3]int{13, 0, 0})
		}
	default:
		err = db.WithContext(ctx).Raw("SELECT VERSION()").Scan(&version).Error
		if err == nil {
			maria := strings.Contains(strings.ToLower(version), "mariadb")
			switch driver {
			case "mysql":
				if maria {
					err = fmt.Errorf("mysql requires a MySQL server, got %s", version)
				} else {
					err = checkMinimumVersion(driver, version, [3]int{8, 0, 16})
				}
			case "mariadb":
				if !maria {
					err = fmt.Errorf("mariadb requires a MariaDB server, got %s", version)
				} else {
					err = checkMinimumVersion(driver, version, [3]int{10, 5, 0})
				}
			}
		}
	}
	if err != nil {
		pool, poolErr := db.DB()
		if poolErr != nil {
			return nil, errors.Join(err, poolErr)
		}
		return nil, errors.Join(err, pool.Close())
	}
	return db, nil
}

// checkMinimumVersion compares the numeric release before vendor suffixes.
func checkMinimumVersion(driver, version string, minimum [3]int) error {
	numeric := version
	if driver == "mariadb" {
		numeric = strings.TrimPrefix(numeric, "5.5.5-")
	}
	if end := strings.IndexFunc(numeric, func(r rune) bool { return (r < '0' || r > '9') && r != '.' }); end >= 0 {
		numeric = numeric[:end]
	}
	var actual [3]int
	for i, part := range strings.SplitN(numeric, ".", 3) {
		value, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("parse %s server version %q: %w", driver, version, err)
		}
		actual[i] = value
	}
	for i, required := range minimum {
		if actual[i] > required {
			return nil
		}
		if actual[i] < required {
			return fmt.Errorf("%s requires version >= %d.%d.%d, got %s", driver, minimum[0], minimum[1], minimum[2], version)
		}
	}
	return nil
}

// productDialector applies product-specific connection settings.
func productDialector(driver, dsn string) (gorm.Dialector, error) {
	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dialector = postgresProduct{postgres.Open(dsn).(*postgres.Dialector)}
	case "sqlite":
		normalized, err := sqliteDSN(dsn)
		if err != nil {
			return nil, err
		}
		dialector = sqliteProduct{sqlite.Open(normalized).(*sqlite.Dialector)}
	case "mysql", "mariadb":
		config, err := mysqldriver.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		config.ParseTime = true
		config.Loc = time.UTC
		if config.Params == nil {
			config.Params = map[string]string{}
		}
		config.Params["time_zone"] = "'+00:00'"
		dialector = mysqlProduct{mysql.Open(config.FormatDSN()).(*mysql.Dialector), driver}
	default:
		return nil, fmt.Errorf("unsupported database product %q", driver)
	}
	return dialector, nil
}

// DriverOf preserves product identity through sessions and transactions.
func DriverOf(db *gorm.DB) string { return db.Dialector.(interface{ Product() string }).Product() }

// Config holds validated pool limits supplied by the application.
type Config struct{ MaxOpenConns, MaxIdleConns int }

func ApplyPool(db *gorm.DB, cfg Config) error {
	pool, err := db.DB()
	if err != nil {
		return err
	}
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	return nil
}
