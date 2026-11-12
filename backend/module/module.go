// Package module contains the public contracts for composing application
// modules. Authentication, configuration loading, and system implementations
// remain application-owned.
package module

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/permission"
	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/storage"
)

// Runtime is the shared infrastructure supplied by the application to a
// module constructor. DB and Logger are borrowed; a module closes only
// resources it creates itself. Config is a value copy owned by the application.
type Runtime struct {
	DB        *gorm.DB
	Logger    *slog.Logger
	Config    adminconfig.Config
	Context   context.Context
	DataScope resource.DataScopeProvider
	// Storage is the application's object storage, shared by every module.
	// It is borrowed: modules must not close it.
	Storage storage.Storage
	// AccessLog is the request log dispatcher, or nil when access logging
	// is disabled. Modules read stored entries through AccessLog.Reader() and
	// enrich the current request with accesslog.SetAttr. It is borrowed:
	// modules must not close it.
	AccessLog *accesslog.Logger
}

// Definition declares one module. Static Services, Permissions, HTTP, and
// declarations are validated before Create runs. Business-specific configuration
// belongs in the Create closure rather than in Runtime.
type Definition struct {
	Name        string
	Requires    []string
	Services    []ConnectService
	Permissions func(permission.Registrar) error
	HTTP        []HTTPRoute
	Create      func(context.Context, Runtime) (Instance, error)
}

// Instance contains resources created by a module.
type Instance struct {
	Services []ConnectService
	HTTP     []HTTPHandler
	Close    func(context.Context) error
}
