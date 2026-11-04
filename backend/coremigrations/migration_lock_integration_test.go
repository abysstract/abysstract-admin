//go:build integration

package coremigrations_test

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/abysstract/abysstract-admin/backend/coremigrations"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestMigrationLockAndForwardUpgrade(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	pool.SetMaxIdleConns(2)
	if cfg.Driver == "sqlite" {
		pool.SetMaxOpenConns(1)
		pool.SetMaxIdleConns(1)
	}
	if err := testdb.Verify(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := testdb.ResetSchema(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	sources := fstest.MapFS{"20261001090001_probe.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE p2_migration_probe (id integer PRIMARY KEY);\nINSERT INTO p2_migration_probe VALUES (1);\n")}}
	release := acquireFixtureLock(t, pool, cfg)
	start := time.Now()
	if err := coremigrations.Run(context.Background(), db, sources); err == nil || !strings.Contains(err.Error(), "lock timeout") {
		t.Fatalf("expected timeout: %v", err)
	}
	if time.Since(start) < 300*time.Millisecond {
		t.Fatal("wait deadline")
	}
	if db.Migrator().HasTable("goose_db_version") {
		t.Fatal("provider ran before lock")
	}
	release()
	for i := 0; i < 2; i++ {
		if err := coremigrations.Run(context.Background(), db, sources); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("p2_migration_probe").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("repeat count %d %v", count, err)
	}
	sources["20261001090002_upgrade.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nINSERT INTO p2_migration_probe VALUES (2);\n")}
	if err := coremigrations.Run(context.Background(), db, sources); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("cancelled Go migration")
	ctx, cancel := context.WithCancel(context.Background())
	failure := goose.NewGoMigration(20261001090003, &goose.GoFunc{RunTx: func(context.Context, *sql.Tx) error { cancel(); return sentinel }}, nil)
	if err := coremigrations.Run(ctx, db, sources, failure); err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("business error %v", err)
	}
	// Independent cleanup context releases the session lock after cancellation.
	if err := coremigrations.Run(context.Background(), db, sources); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("p2_migration_probe").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("upgrade count %d %v", count, err)
	}
}

// TestMigrationProcessHelper is the child process entry for the lock lifecycle test.
func TestMigrationProcessHelper(t *testing.T) {
	mode := os.Getenv("ADMIN_P2_MIGRATION_PROCESS")
	if mode == "" {
		return
	}
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	pool.SetMaxIdleConns(2)
	if cfg.Driver == "sqlite" {
		pool.SetMaxOpenConns(1)
		pool.SetMaxIdleConns(1)
	}
	if mode == "hold" {
		release := acquireFixtureLock(t, pool, cfg)
		fmt.Println("LOCKED")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		release()
		return
	}
	sources := fstest.MapFS{"20261001090001_process.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE p2_process_probe(id integer);\n")}}
	if err := coremigrations.Run(context.Background(), db, sources); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationTwoProcessesAndExitRecovery(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := testdb.Verify(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := testdb.ResetSchema(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	for _, termination := range []string{"release", "kill"} {
		t.Run(termination, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			holder := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMigrationProcessHelper$")
			holder.Env = append(os.Environ(), "ADMIN_P2_MIGRATION_PROCESS=hold")
			input, err := holder.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := holder.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			holder.Stderr = os.Stderr
			if err := holder.Start(); err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			scanner := bufio.NewScanner(output)
			if !scanner.Scan() || scanner.Text() != "LOCKED" {
				t.Fatal("missing lock signal")
			}

			contenderDSN := cfg.DSN
			if cfg.Driver == "sqlite" {
				alias := filepath.Join(t.TempDir(), "alias.sqlite")
				if err := os.Symlink(cfg.Database, alias); err != nil {
					t.Fatal(err)
				}
				contenderDSN = (&url.URL{Scheme: "file", Path: alias}).String()
				run := "66666666666666666666666666666666"
				other, err := testdb.CreateSQLite(ctx, run, filepath.Join(t.TempDir(), "admin_core_k2_"+run))
				if err != nil {
					t.Fatal(err)
				}
				independent, err := testdb.Open(other)
				if err != nil {
					t.Fatal(err)
				}
				ownPool, err := independent.DB()
				if err != nil {
					t.Fatal(err)
				}
				sources := fstest.MapFS{"00001_other.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE independent(id INTEGER);\n")}}
				if err := coremigrations.Run(ctx, independent, sources); err != nil {
					t.Fatal(err)
				}
				if err := ownPool.Close(); err != nil {
					t.Fatal(err)
				}
				if err := testdb.DropSQLite(other); err != nil {
					t.Fatal(err)
				}
				t.Log("holder locked main file; independent file migrated; contender uses symlink to main")
			}
			contender := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMigrationProcessHelper$")
			contender.Env = append(os.Environ(), "ADMIN_P2_MIGRATION_PROCESS=migrate", "ADMIN_TEST_DSN="+contenderDSN)
			logs, err := contender.CombinedOutput()
			if err == nil || !strings.Contains(string(logs), "lock timeout 300ms") {
				t.Fatalf("contender %v %s", err, logs)
			}
			if termination == "kill" {
				if err := holder.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				if err := holder.Wait(); err == nil {
					t.Fatal("kill status")
				}
			} else {
				if _, err := io.WriteString(input, "release\n"); err != nil {
					t.Fatal(err)
				}
				if _, err := io.Copy(io.Discard, output); err != nil {
					t.Fatal(err)
				}
				if err := holder.Wait(); err != nil {
					t.Fatal(err)
				}
			}
			var lockBefore os.FileInfo
			if cfg.Driver == "sqlite" {
				lockBefore, err = os.Stat(cfg.Database + ".migrate.lock")
				if err != nil {
					t.Fatal(err)
				}
			}

			retry := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMigrationProcessHelper$")
			retry.Env = append(os.Environ(), "ADMIN_P2_MIGRATION_PROCESS=migrate")
			if logs, err := retry.CombinedOutput(); err != nil {
				t.Fatalf("retry: %v %s", err, logs)
			}
			if cfg.Driver == "sqlite" {
				after, err := os.Stat(cfg.Database + ".migrate.lock")
				if err != nil || !os.SameFile(lockBefore, after) {
					t.Fatalf("stable lock inode: %v", err)
				}
				t.Log("lock inode retained after recovery")
			}

		})
	}
}
