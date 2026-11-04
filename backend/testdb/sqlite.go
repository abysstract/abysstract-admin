package testdb

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
)

// CreateSQLite provisions a new exclusive directory and marks its file target.
func CreateSQLite(ctx context.Context, runID, directory string) (Config, error) {
	name, err := DatabaseName(runID)
	if err != nil {
		return Config{}, err
	}
	if !filepath.IsAbs(directory) || filepath.Base(directory) != name {
		return Config{}, errors.New("testdb: SQLite directory must be absolute and match the run ID")
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return Config{}, err
	}
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(canonical, "admin.sqlite")
	c := Config{Driver: "sqlite", Database: path, RunID: runID, DSN: (&url.URL{Scheme: "file", Path: path}).String()}
	if err := os.WriteFile(filepath.Join(canonical, ".owner"), []byte(c.Marker()), 0600); err != nil {
		return Config{}, err
	}
	db, err := Open(c)
	if err != nil {
		return Config{}, err
	}
	pool, err := db.DB()
	if err != nil {
		return Config{}, err
	}
	return c, pool.Close()
}

func (c Config) validateSQLite() error {
	name, err := DatabaseName(c.RunID)
	if err != nil {
		return err
	}
	path, err := database.SQLitePath(c.DSN)
	if err != nil {
		return errors.New("testdb: invalid SQLite file URI")
	}
	if path != c.Database || filepath.Base(path) != "admin.sqlite" || filepath.Base(filepath.Dir(path)) != name {
		return errors.New("testdb: SQLite path differs from the authorized target")
	}
	marker, err := os.ReadFile(filepath.Join(filepath.Dir(path), ".owner"))
	if err != nil {
		return err
	}
	if string(marker) != c.Marker() {
		return errors.New("testdb: SQLite ownership marker differs from this run")
	}
	return nil
}

func verifySQLite(ctx context.Context, db *gorm.DB, c Config) error {
	var attached []struct{ Name, File string }
	if err := db.WithContext(ctx).Raw("PRAGMA database_list").Scan(&attached).Error; err != nil {
		return err
	}
	for _, entry := range attached {
		if entry.Name == "main" && entry.File == c.Database {
			return nil
		}
	}
	return errors.New("testdb: connected SQLite file differs from the authorized target")
}

// DropSQLite rechecks ownership immediately before removing this run's directory.
func DropSQLite(c Config) error {
	db, err := Open(c)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	if err := pool.Close(); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Dir(c.Database))
}
