// admin-testdb provisions and verifies explicitly authorized disposable test
// databases. Every action requires an explicit action and dedicated inputs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func main() {
	action := flag.String("action", "", "create, verify, or drop")
	runID := flag.String("run-id", "", "32-character lowercase hexadecimal run ID (create)")
	output := flag.String("dsn-file", "", "new private output file (create)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := run(ctx, *action, *runID, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, action, runID, output string) error {
	switch action {
	case "create":
		switch os.Getenv("ADMIN_TEST_DRIVER") {
		case "sqlite":
			return createSQLite(ctx, runID, output)
		case "mysql", "mariadb":
			return createMySQL(ctx, runID, output)
		case "", "postgres":
			return create(ctx, runID, output)
		default:
			return errors.New("testdb: ADMIN_TEST_DRIVER must be postgres, mysql, mariadb or sqlite")
		}
	case "verify", "drop":
		return verifyOrDrop(ctx, action)
	default:
		return errors.New("testdb: choose an explicit create, verify, or drop action")
	}
}

func create(ctx context.Context, runID, output string) (result error) {
	name, err := testdb.DatabaseName(runID)
	if err != nil {
		return err
	}
	c, err := testdb.ForDatabase(os.Getenv("ADMIN_TEST_BASE_DSN"), name, runID)
	if err != nil {
		return err
	}
	if output == "" {
		return errors.New("testdb: new DSN output file is required")
	}
	// O_EXCL protects existing files and symlinks before any database operation.
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("testdb: output file must be a new private file")
	}
	keep := false
	defer func() {
		if err := file.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("testdb: export close failed; newly created %s requires cleanup", name))
			keep = false
		}
		if !keep {
			if err := os.Remove(output); err != nil {
				result = errors.Join(result, errors.New("testdb: private output cleanup failed"))
			}
		}
	}()
	conn, err := pgx.Connect(ctx, os.Getenv("ADMIN_TEST_BASE_DSN"))
	if err != nil {
		return errors.New("testdb: explicit base connection failed")
	}
	defer conn.Close(ctx)
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		return errors.New("testdb: create failed; existing databases are preserved")
	}
	if _, err := conn.Exec(ctx, "COMMENT ON DATABASE "+quoted+" IS '"+c.Marker()+"'"); err != nil {
		return fmt.Errorf("testdb: marker write failed; newly created %s requires cleanup", name)
	}
	// Open parses and dials c.DSN, exactly the bytes exported to tests.
	db, err := testdb.Open(c)
	if err != nil {
		return fmt.Errorf("testdb: verification failed; newly created %s requires cleanup", name)
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("testdb: verified pool unavailable")
	}
	if err := pool.Close(); err != nil {
		return errors.New("testdb: verified pool close failed")
	}
	if _, err := file.WriteString(c.DSN); err != nil {
		return fmt.Errorf("testdb: export failed; newly created %s requires cleanup", name)
	}
	keep = true
	fmt.Println("created and verified", name)
	return nil
}

func createSQLite(ctx context.Context, runID, output string) error {
	name, err := testdb.DatabaseName(runID)
	if err != nil {
		return err
	}
	if output == "" {
		return errors.New("testdb: new DSN output file is required")
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	c, err := testdb.CreateSQLite(ctx, runID, filepath.Join(filepath.Dir(output), name))
	if err != nil {
		return errors.Join(err, file.Close(), os.Remove(output))
	}
	_, err = file.WriteString(c.DSN)
	return errors.Join(err, file.Close())
}

func verifyOrDrop(ctx context.Context, action string) error {
	c, err := testdb.FromEnv()
	if err != nil {
		return err
	}
	if c.Driver == "sqlite" && action == "drop" {
		return testdb.DropSQLite(c)
	}
	db, err := testdb.Open(c)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("testdb: pool unavailable")
	}
	if (c.Driver == "mysql" || c.Driver == "mariadb") && action == "drop" {
		if err := testdb.Verify(ctx, db, c); err != nil {
			return errors.Join(err, pool.Close())
		}
		err := db.WithContext(ctx).Exec("DROP DATABASE `" + c.Database + "`").Error
		err = errors.Join(err, pool.Close())
		if err != nil {
			return errors.New("testdb: drop failed")
		}
		fmt.Println("removed", c.Database)
		return nil
	}
	if err := pool.Close(); err != nil {
		return errors.New("testdb: verification pool close failed")
	}
	if action == "verify" {
		fmt.Println("verified", c.Database)
		return nil
	}
	// The same DSN supplies host/authentication; only the database changes to
	// the maintenance database. Recheck ownership there immediately before DROP.
	cfg, err := pgx.ParseConfig(c.DSN)
	if err != nil {
		return errors.New("testdb: invalid DSN")
	}
	cfg.Database = "postgres"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("testdb: maintenance connection failed")
	}
	defer conn.Close(ctx)
	var marker string
	if err := conn.QueryRow(ctx, "SELECT COALESCE(shobj_description(oid,'pg_database'),'') FROM pg_database WHERE datname=$1", c.Database).Scan(&marker); err != nil {
		return errors.New("testdb: ownership recheck failed")
	}
	if marker != c.Marker() {
		return errors.New("testdb: ownership changed before cleanup")
	}
	if _, err := conn.Exec(ctx, "DROP DATABASE "+pgx.Identifier{c.Database}.Sanitize()); err != nil {
		return errors.New("testdb: drop failed")
	}
	fmt.Println("removed", c.Database)
	return nil
}
