package adminapp

import (
	"context"
	"net"

	"gorm.io/gorm"

	db "github.com/abysstract/abysstract-admin/backend/database"
)

var openDatabase = db.Open

var applyPool = db.ApplyPool

var listen = net.Listen

var pingDatabase = func(ctx context.Context, gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

var requireSystemTables = func(gdb *gorm.DB) error {
	if gdb == nil || !gdb.Migrator().HasTable("sys_user") {
		return errMissingSystemTables
	}
	return nil
}

var errMissingSystemTables = errString("adminapp: database is missing sys_user; run migrate first")

type errString string

func (e errString) Error() string { return string(e) }
