package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidRequestsRejectOffline(t *testing.T) {
	t.Setenv("ADMIN_TEST_DSN", "")
	t.Setenv("ADMIN_TEST_DATABASE", "")
	t.Setenv("ADMIN_TEST_RUN_ID", "")
	t.Setenv("ADMIN_TEST_BASE_DSN", "")
	for _, action := range []string{"", "unknown", "create", "verify", "drop"} {
		if err := run(context.Background(), action, "", ""); err == nil {
			t.Fatalf("action %q admitted incomplete inputs", action)
		}
	}
}

func TestCreatePreservesExistingOutputBeforeConnection(t *testing.T) {
	t.Setenv("ADMIN_TEST_BASE_DSN", "host=invalid.example dbname=postgres sslmode=disable")
	path := filepath.Join(t.TempDir(), "dsn")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := create(context.Background(), "0123456789abcdef0123456789abcdef", path); err == nil {
		t.Fatal("existing file accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "keep" {
		t.Fatal("existing file changed")
	}
}

func TestCreateRejectsUnsupportedDriverBeforeConnection(t *testing.T) {
	t.Setenv("ADMIN_TEST_BASE_DSN", "host=invalid.example dbname=postgres sslmode=disable")
	for _, driver := range []string{"unknown"} {
		t.Setenv("ADMIN_TEST_DRIVER", driver)
		path := filepath.Join(t.TempDir(), "dsn")
		err := run(context.Background(), "create", "0123456789abcdef0123456789abcdef", path)
		if err == nil || !strings.Contains(err.Error(), "ADMIN_TEST_DRIVER") {
			t.Fatalf("driver=%s error=%v", driver, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unsupported driver wrote output: %v", err)
		}
	}
}
