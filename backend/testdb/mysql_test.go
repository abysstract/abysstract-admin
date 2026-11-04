package testdb

import (
	"testing"

	"gorm.io/gorm"
)

func TestMySQLDSNAdmission(t *testing.T) {
	name, _ := DatabaseName(runID)
	cfg := Config{Driver: "mysql", DSN: "user:secret@tcp(localhost:3306)/" + name, Database: name, RunID: runID}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.DSN = "user:secret@tcp(localhost:3306)/other"
	calls := 0
	if _, err := open(cfg, func(string) (*gorm.DB, error) { calls++; return nil, nil }, func(*gorm.DB) error { calls++; return nil }); err == nil || calls != 0 {
		t.Fatal("wrong target dialed")
	}
}
