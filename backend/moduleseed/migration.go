package moduleseed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Migration embeds a declaration snapshot so history survives module removal.
func Migration(version int64, driver, action, snapshot string, previous ...string) *goose.Migration {
	return goose.NewGoMigration(version, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
		spec, err := Parse([]byte(snapshot))
		if err != nil {
			return err
		}
		var dialector gorm.Dialector
		switch driver {
		case "postgres":
			dialector = postgres.New(postgres.Config{Conn: tx})
		case "mysql", "mariadb":
			dialector = mysql.New(mysql.Config{Conn: tx, SkipInitializeWithVersion: true})
		case "sqlite":
			dialector = sqlite.New(sqlite.Config{Conn: tx})
		default:
			return fmt.Errorf("seed: unsupported driver %s", driver)
		}
		db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
		if err != nil {
			return err
		}
		db = db.WithContext(ctx)
		switch action {
		case "install":
			return Install(db, spec)
		case "upgrade":
			if len(previous) != 1 {
				return fmt.Errorf("seed: upgrade requires a previous snapshot")
			}
			before, err := Parse([]byte(previous[0]))
			if err != nil {
				return err
			}
			return Upgrade(db, before, spec)
		case "remove":
			return Remove(db, spec.Module)
		default:
			return fmt.Errorf("seed: unknown action %s", action)
		}
	}}, nil)
}
