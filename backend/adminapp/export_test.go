package adminapp

import (
	"context"
	"net"

	"gorm.io/gorm"

	db "github.com/abysstract/abysstract-admin/backend/database"
)

func OverrideOpenDatabase(fn func(context.Context, string, string) (*gorm.DB, error)) func() {
	previous := openDatabase
	openDatabase = fn
	return func() { openDatabase = previous }
}

func OverrideListen(fn func(network, address string) (net.Listener, error)) func() {
	previous := listen
	listen = fn
	return func() { listen = previous }
}

func OverrideRequireSystemTables(fn func(*gorm.DB) error) func() {
	previous := requireSystemTables
	requireSystemTables = fn
	return func() { requireSystemTables = previous }
}

func OverridePingDatabase(fn func(context.Context, *gorm.DB) error) func() {
	previous := pingDatabase
	pingDatabase = fn
	return func() { pingDatabase = previous }
}

func OverrideApplyPool(fn func(*gorm.DB, db.Config) error) func() {
	previous := applyPool
	applyPool = fn
	return func() { applyPool = previous }
}
