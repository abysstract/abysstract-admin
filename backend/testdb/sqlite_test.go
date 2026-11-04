//go:build integration

package testdb_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestSQLiteAdmissionAndCleanup(t *testing.T) {
	run := "44444444444444444444444444444444"
	dir := filepath.Join(t.TempDir(), "admin_core_k2_"+run)
	cfg, err := testdb.CreateSQLite(context.Background(), run, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_DB_DSN", "file:/outside/application.sqlite")
	for _, item := range []struct {
		name   string
		change func(*testdb.Config)
	}{
		{"missing DSN", func(c *testdb.Config) { c.DSN = "" }},
		{"different path", func(c *testdb.Config) { c.Database += ".other" }},
		{"different run", func(c *testdb.Config) { c.RunID = "55555555555555555555555555555555" }},
	} {
		t.Run(item.name, func(t *testing.T) {
			c := cfg
			item.change(&c)
			if _, err := testdb.Open(c); err == nil {
				t.Fatal("unowned target admitted")
			}
		})
	}
	marker := filepath.Join(filepath.Dir(cfg.Database), ".owner")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Open(cfg); err == nil {
		t.Fatal("missing marker admitted")
	}
	if err := testdb.DropSQLite(cfg); err == nil {
		t.Fatal("unowned cleanup allowed")
	}
	if _, err := os.Stat(cfg.Database); err != nil {
		t.Fatal("rejected cleanup changed file", err)
	}
	if err := os.WriteFile(marker, []byte(cfg.Marker()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := testdb.DropSQLite(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(cfg.Database)); !os.IsNotExist(err) {
		t.Fatal("owned directory remains", err)
	}
}
