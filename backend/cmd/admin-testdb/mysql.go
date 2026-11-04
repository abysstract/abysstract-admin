package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func createMySQL(ctx context.Context, runID, output string) (result error) {
	name, err := testdb.DatabaseName(runID)
	if err != nil {
		return err
	}
	raw := os.Getenv("ADMIN_TEST_BASE_DSN")
	if raw == "" {
		return errors.New("testdb: explicit base DSN is required")
	}
	base, err := mysqldriver.ParseDSN(raw)
	if err != nil {
		return errors.New("testdb: invalid base DSN")
	}
	base.DBName = name
	cfg := testdb.Config{Driver: os.Getenv("ADMIN_TEST_DRIVER"), DSN: base.FormatDSN(), Database: name, RunID: runID}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if output == "" {
		return errors.New("testdb: new DSN output file is required")
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("testdb: output file must be a new private file")
	}
	keep := false
	defer func() {
		result = errors.Join(result, file.Close())
		if !keep || result != nil {
			result = errors.Join(result, os.Remove(output))
		}
	}()
	base.DBName = ""
	pool, err := sql.Open("mysql", base.FormatDSN())
	if err != nil {
		return errors.New("testdb: base connection failed")
	}
	defer func() { result = errors.Join(result, pool.Close()) }()
	if err := verifyMySQLProduct(ctx, pool, cfg.Driver); err != nil {
		return err
	}
	if _, err := pool.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin"); err != nil {
		return errors.New("testdb: create failed; existing databases are preserved")
	}
	if _, err := pool.ExecContext(ctx, "CREATE TABLE `"+name+"`.p0_owner(run_id CHAR(32) PRIMARY KEY) ENGINE=InnoDB"); err != nil {
		return fmt.Errorf("testdb: marker creation failed; newly created %s requires cleanup", name)
	}
	if _, err := pool.ExecContext(ctx, "INSERT INTO `"+name+"`.p0_owner VALUES (?)", runID); err != nil {
		return fmt.Errorf("testdb: marker write failed; newly created %s requires cleanup", name)
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		return fmt.Errorf("testdb: verification failed; newly created %s requires cleanup", name)
	}
	admitted, err := db.DB()
	if err != nil {
		return err
	}
	if err := admitted.Close(); err != nil {
		return err
	}
	if _, err := file.WriteString(cfg.DSN); err != nil {
		return err
	}
	keep = true
	fmt.Println("created and verified", name)
	return nil
}

func verifyMySQLProduct(ctx context.Context, pool *sql.DB, driver string) error {
	var version string
	if err := pool.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return errors.New("testdb: base connection failed")
	}
	maria := strings.Contains(strings.ToLower(version), "mariadb")
	if driver == "mysql" && (maria || !strings.HasPrefix(version, "8.4.")) {
		return errors.New("testdb: mysql requires 8.4")
	}
	if driver == "mariadb" && (!maria || !strings.HasPrefix(version, "11.4.")) {
		return errors.New("testdb: mariadb requires 11.4")
	}
	return nil
}
