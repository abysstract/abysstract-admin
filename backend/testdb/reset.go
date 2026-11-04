package testdb

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ResetSchema clears the admitted target while retaining its ownership marker.
func ResetSchema(ctx context.Context, db *gorm.DB, cfg Config) error {
	if err := Verify(ctx, db, cfg); err != nil {
		return err
	}
	if cfg.Driver == "sqlite" {
		return db.WithContext(ctx).Connection(func(conn *gorm.DB) (err error) {
			if err := Verify(ctx, conn, cfg); err != nil {
				return err
			}
			if err := conn.Exec("PRAGMA foreign_keys=OFF").Error; err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, conn.WithContext(context.Background()).Exec("PRAGMA foreign_keys=ON").Error)
			}()
			var tables []string
			if err := conn.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tables).Error; err != nil {
				return err
			}
			for _, table := range tables {
				if err := conn.Exec("DROP TABLE \"" + strings.ReplaceAll(table, "\"", "\"\"") + "\"").Error; err != nil {
					return err
				}
			}
			return Verify(ctx, conn, cfg)
		})
	}
	if cfg.Driver != "mysql" && cfg.Driver != "mariadb" {
		if err := db.WithContext(ctx).Exec("DROP SCHEMA public CASCADE").Error; err != nil {
			return err
		}
		return db.WithContext(ctx).Exec("CREATE SCHEMA public").Error
	}
	return db.WithContext(ctx).Connection(func(conn *gorm.DB) (err error) {
		if err := Verify(ctx, conn, cfg); err != nil {
			return err
		}
		if err := conn.Exec("SET FOREIGN_KEY_CHECKS=0").Error; err != nil {
			return err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, conn.WithContext(cleanup).Exec("SET FOREIGN_KEY_CHECKS=1").Error)
		}()
		var tables []string
		if err := conn.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name <> 'p0_owner' ORDER BY table_name").Scan(&tables).Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := conn.Exec("DROP TABLE `" + strings.ReplaceAll(table, "`", "``") + "`").Error; err != nil {
				return err
			}
		}
		return Verify(ctx, conn, cfg)
	})
}
