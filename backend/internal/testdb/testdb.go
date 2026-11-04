// Package testdb provides the mandatory isolated PostgreSQL fixture for the
// public transaction tests. It intentionally fails when the caller did not supply
// the dedicated database DSN; a skipped test cannot prove transaction
// behavior.
package testdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	guard "github.com/abysstract/abysstract-admin/backend/testdb"
)

var schemaCounter atomic.Uint64

// Database owns one schema in the caller-provided one-time database.
type Database struct {
	DB      *gorm.DB
	Schema  string
	Product string
	Event   string
}

type fixtureCleanup struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	schema string
	tables []string
	config guard.Config
}

func (c *fixtureCleanup) run(ctx context.Context) error {
	return cleanupResources(ctx, func(ctx context.Context) error {
		if err := guard.Verify(ctx, c.db, c.config); err != nil {
			return err
		}
		if c.schema == "" {
			for _, table := range c.tables {
				if err := c.db.WithContext(ctx).Exec("DROP TABLE " + table).Error; err != nil {
					return err
				}
			}
			return nil
		}
		return c.db.WithContext(ctx).Exec(`DROP SCHEMA ` + quoteIdentifier(c.schema) + ` CASCADE`).Error
	}, c.sqlDB.Close)
}

// cleanupResources runs every cleanup action and preserves each error. The
// fixture uses it so a failed schema drop cannot hide a failed pool close, and
// one failed action cannot prevent the remaining action from running.
func cleanupResources(ctx context.Context, dropSchema func(context.Context) error, closePool func() error) error {
	var cleanupErrors []error
	if dropSchema != nil {
		if err := dropSchema(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("drop fixture schema: %w", err))
		}
	}
	if closePool != nil {
		if err := closePool(); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("close fixture pool: %w", err))
		}
	}
	return errors.Join(cleanupErrors...)
}

// Open admits the dedicated test DSN and this run's ownership marker, and
// creates only tables owned by this fixture. The schema is removed during test
// cleanup. The DSN is never printed.
func Open(t *testing.T) *Database {
	t.Helper()
	config, err := guard.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	gdb, err := guard.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL pool: %v", err)
	}
	cleanup := &fixtureCleanup{db: gdb, sqlDB: sqlDB, config: config}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cleanup.run(cleanupContext); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	})

	schema := fmt.Sprintf("admin_core_k2_%d_%d", os.Getpid(), schemaCounter.Add(1))
	product := qualified(schema, "products")
	event := qualified(schema, "events")
	idType := "BIGSERIAL PRIMARY KEY"
	textType := "TEXT"
	deferred := " DEFERRABLE INITIALLY DEFERRED"
	if config.Driver != "postgres" {
		product = schema + "_products"
		event = schema + "_events"
		cleanup.tables = []string{event, product}
		idType = "BIGINT AUTO_INCREMENT PRIMARY KEY"
		textType = "VARCHAR(255)"
		deferred = ""
		if config.Driver == "sqlite" {
			idType = "INTEGER PRIMARY KEY AUTOINCREMENT"
			textType = "TEXT"
		}
	} else {
		if err := gdb.Exec("CREATE SCHEMA " + quoteIdentifier(schema)).Error; err != nil {
			t.Fatal(err)
		}
		cleanup.schema = schema
	}
	if err := gdb.Exec("CREATE TABLE " + product + " (id " + idType + ",name " + textType + " NOT NULL UNIQUE)").Error; err != nil {
		t.Fatal(err)
	}
	extra := ""
	if config.Driver == "sqlite" {
		extra = ", commit_guard BIGINT REFERENCES " + product + "(id) DEFERRABLE INITIALLY DEFERRED"
	}
	if err := gdb.Exec("CREATE TABLE " + event + " (id " + idType + ", product_name " + textType + " NOT NULL,event_key " + textType + " NOT NULL" + extra + ", UNIQUE(event_key)" + deferred + ")").Error; err != nil {
		t.Fatal(err)
	}

	return &Database{DB: gdb, Schema: schema, Product: product, Event: event}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func qualified(schema, table string) string {
	return quoteIdentifier(schema) + "." + quoteIdentifier(table)
}

// SeedProduct inserts a known product row for a database-error test.
func (d *Database) SeedProduct(t *testing.T, name string) {
	t.Helper()
	if err := d.DB.Exec("INSERT INTO "+d.Product+" (name) VALUES (?)", name).Error; err != nil {
		t.Fatalf("seed product %q: %v", name, err)
	}
}

// SeedEvent inserts a known event key for a deferred-constraint test.
func (d *Database) SeedEvent(t *testing.T, productName, eventKey string) {
	t.Helper()
	if err := d.DB.Exec("INSERT INTO "+d.Event+" (product_name, event_key) VALUES (?, ?)", productName, eventKey).Error; err != nil {
		t.Fatalf("seed event %q: %v", eventKey, err)
	}
}

// TableName makes a fixture table usable by GORM without relying on a mutable
// search_path shared by pooled connections.
func (d *Database) TableName(table string) string {
	switch table {
	case "products":
		return d.Product
	case "events":
		return d.Event
	default:
		panic("unknown fixture table " + strconv.Quote(table))
	}
}
