package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

func SeedAdminMigration() *goose.Migration {
	return goose.NewGoMigration(20261020090002, &goose.GoFunc{RunTx: upSeedAdmin}, nil)
}

func upSeedAdmin(ctx context.Context, tx *sql.Tx) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sys_user (id, dept_id, username, nickname, email, password, status, remark, create_time, update_time)
		VALUES (1, 1, 'admin', '超级管理员', 'admin@abysstract.local', $1, 1, '内置管理员', $2, $2)`, string(hash), now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sys_user_role (user_id, role_id) VALUES (1, 1)`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('sys_user', 'id'), (SELECT MAX(id) FROM sys_user))`)
	return err
}
