package database

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sqliteProduct struct{ *sqlite.Dialector }

func (sqliteProduct) Product() string { return "sqlite" }
func (sqliteProduct) Translate(err error) error {
	var native sqlite3.Error
	if errors.As(err, &native) && (native.ExtendedCode == sqlite3.ErrConstraintUnique || native.ExtendedCode == sqlite3.ErrConstraintPrimaryKey) {
		return gorm.ErrDuplicatedKey
	}
	return err
}

// SQLitePath returns the canonical local file identity used by connections and locks.
func SQLitePath(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" || u.Host != "" || u.Opaque != "" || !filepath.IsAbs(u.Path) || u.Fragment != "" {
		return "", fmt.Errorf("sqlite requires an absolute local file URI")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", err
	}
	if q.Get("mode") == "memory" || q.Get("mode") == "ro" {
		return "", fmt.Errorf("sqlite requires a writable persistent file")
	}
	path, err := filepath.EvalSymlinks(u.Path)
	if errors.Is(err, os.ErrNotExist) {
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(u.Path))
		if parentErr != nil {
			return "", parentErr
		}
		return filepath.Join(parent, filepath.Base(u.Path)), nil
	}
	return path, err
}

func sqliteDSN(dsn string) (string, error) {
	path, err := SQLitePath(dsn)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for _, key := range []string{"_journal", "_fk", "_timeout", "_locking_mode", "_locking", "cache", "mode"} {
		q.Del(key)
	}
	q.Set("_journal_mode", "WAL")
	q.Set("_foreign_keys", "on")
	q.Set("_busy_timeout", "250")
	q.Set("_txlock", "immediate")
	u.Path, u.RawPath, u.RawQuery = path, "", q.Encode()
	return u.String(), nil
}
