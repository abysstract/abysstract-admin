package adminapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	db "github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var columnNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type dataScopeAdapter struct {
	repo *repo.Repo
}

func (a dataScopeAdapter) DataScope(ctx context.Context, deptDBName, creatorDBName string) resource.AccessScope {
	if a.repo == nil || !columnNamePattern.MatchString(deptDBName) || !columnNamePattern.MatchString(creatorDBName) {
		return resource.AccessScope{}
	}
	return a.repo.DataScopeAccessScope(ctx, deptDBName, creatorDBName)
}

func acquireDB(ctx context.Context, cfg adminconfig.Config, borrowed *gorm.DB) (*gorm.DB, bool, error) {
	if borrowed != nil {
		if err := pingDatabase(ctx, borrowed); err != nil {
			return nil, false, fmt.Errorf("adminapp: ping borrowed database: %w", err)
		}
		return borrowed, false, nil
	}
	gdb, err := openDatabase(ctx, cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return nil, false, fmt.Errorf("adminapp: open database: %w", err)
	}
	if err := applyPool(gdb, db.Config{MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns}); err != nil {
		_ = closeGorm(gdb)
		return nil, false, fmt.Errorf("adminapp: apply database pool: %w", err)
	}
	if err := pingDatabase(ctx, gdb); err != nil {
		_ = closeGorm(gdb)
		return nil, false, fmt.Errorf("adminapp: ping database: %w", err)
	}
	return gdb, true, nil
}

func closeGorm(gdb *gorm.DB) error {
	if gdb == nil {
		return nil
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func defaultLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func requestMaxBytes(cfg adminconfig.Config) int64 {
	return cfg.UploadMaxBytes + 1024*1024
}
