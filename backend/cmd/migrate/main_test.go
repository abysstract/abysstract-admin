package main

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

func TestMigrateValidatesDatabaseWithoutServerSecret(t *testing.T) {
	cfg := &adminconfig.Config{Env: adminconfig.EnvProd, DBDriver: "postgres", DBDSN: "host=localhost user=test dbname=test sslmode=disable", DBMaxOpenConns: 5, DBMaxIdleConns: 2}
	if err := cfg.Validate(); err == nil {
		t.Fatal("server config should require JWT secret")
	}
	if err := cfg.ValidateDatabase(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateRejectsUnknownCommandsBeforeConnection(t *testing.T) {
	for _, args := range [][]string{{"inspect"}, {"adopt", "digest"}, {"run", "extra"}} {
		if err := execute(args, &adminconfig.Config{}); err == nil || !strings.Contains(err.Error(), "usage: migrate") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestMigrateRejectsEmptyDSN(t *testing.T) {
	if err := execute(nil, &adminconfig.Config{DBMaxOpenConns: 5, DBMaxIdleConns: 2}); err == nil || !strings.Contains(err.Error(), "ADMIN_DB_DSN") {
		t.Fatalf("err=%v", err)
	}
}
