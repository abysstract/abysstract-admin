package coremigrations

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func runSQLite(ctx context.Context, db *gorm.DB, sources fs.FS, selected ...*goose.Migration) (err error) {
	var files []struct{ Name, File string }
	if err := db.WithContext(ctx).Raw("PRAGMA database_list").Scan(&files).Error; err != nil {
		return err
	}
	var path string
	for _, file := range files {
		if file.Name == "main" {
			path = file.File
		}
	}
	// Open has already canonicalized the local file; re-resolve aliases for lock identity.
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".migrate.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Close()) }()
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
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
	defer func() { err = errors.Join(err, syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)) }()
	pool, err := db.DB()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, pool, sources, goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(selected...))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
