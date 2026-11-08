package app

import (
	"context"
	"errors"
	"net"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"

	// Enables ADMIN_STORAGE_DRIVER=s3. Remove the import to drop the AWS SDK
	// from the binary when only local storage is used.
	_ "github.com/abysstract/abysstract-admin/backend/storage/s3storage"
)

// New assembles the modules used by this application.
func New(cfg adminconfig.Config) (*adminapp.App, error) {
	modules, err := Modules()
	if err != nil {
		return nil, err
	}
	return adminapp.New(adminapp.Options{Config: cfg, Modules: modules})
}

// Run initializes the modules, serves requests, and then closes their resources.
func Run(ctx context.Context, cfg adminconfig.Config) (err error) {
	core, err := New(cfg)
	if err != nil {
		return err
	}
	handler, err := core.Start(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, core.Shutdown(context.Background())) }()

	server := NewServer(cfg, handler)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	return serve(ctx, server, listener, cfg.ShutdownTimeout)
}
