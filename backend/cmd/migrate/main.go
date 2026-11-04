// Command migrate applies the bundled system migrations to a new database.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	db "github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/migrations"
)

func main() {
	if err := execute(os.Args[1:], adminconfig.Load()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(args []string, cfg *adminconfig.Config) error {
	if len(args) > 1 || len(args) == 1 && args[0] != "run" {
		return fmt.Errorf("usage: migrate [run]")
	}
	if err := cfg.ValidateDatabase(); err != nil {
		return err
	}
	g, err := db.Open(context.Background(), cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if err := db.ApplyPool(g, db.Config{MaxOpenConns: 2, MaxIdleConns: 2}); err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	pool, err := g.DB()
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var database string
	if cfg.DBDriver == "sqlite" {
		pool.SetMaxOpenConns(1)
		pool.SetMaxIdleConns(1)
		database, err = db.SQLitePath(cfg.DBDSN)
		if err != nil {
			return err
		}
	}
	query := "SELECT current_database()"
	if cfg.DBDriver == "mysql" || cfg.DBDriver == "mariadb" {
		query = "SELECT DATABASE()"
	}
	if cfg.DBDriver != "sqlite" {
		if err := g.WithContext(ctx).Raw(query).Scan(&database).Error; err != nil {
			return fmt.Errorf("read database name: %w", err)
		}
	}
	fmt.Fprintf(os.Stderr, "migrate target database: %s\n", database)
	if err := migrations.Run(ctx, g); err != nil {
		return err
	}
	fmt.Println("migrate OK")
	return nil
}
