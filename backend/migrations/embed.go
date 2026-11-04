package migrations

import (
	"context"
	"embed"
	"io/fs"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/coremigrations"
	"github.com/abysstract/abysstract-admin/backend/database"
)

//go:embed postgres/*.sql mysql/*.sql mariadb/*.sql sqlite/*.sql
var sqlFiles embed.FS

func Run(ctx context.Context, db *gorm.DB) error {
	selected, err := fs.Sub(sqlFiles, database.DriverOf(db))
	if err != nil {
		return err
	}
	return coremigrations.Run(ctx, db, selected, goMigrations(database.DriverOf(db))...)
}
