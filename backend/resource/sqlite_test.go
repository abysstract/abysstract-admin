//go:build integration

package resource_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestSQLiteImmediateTransactionLifecycle(t *testing.T) {
	ctx := context.Background()
	run := "33333333333333333333333333333333"
	cfg, err := testdb.CreateSQLite(ctx, run, filepath.Join(t.TempDir(), "admin_core_k2_"+run))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testdb.DropSQLite(cfg); err != nil {
			t.Error(err)
		}
	}()
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Exec("CREATE TABLE lifecycle(id INTEGER PRIMARY KEY, value BIGINT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	holder := db.Begin()
	if holder.Error != nil {
		t.Fatal(holder.Error)
	}
	defer holder.Rollback()
	if err := holder.Exec("INSERT INTO lifecycle VALUES(1,0)").Error; err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	attempted := make(chan struct{})
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		close(attempted)
		done <- store.InTransaction(ctx, func(tx resource.Tx) error {
			callbacks++
			return tx.DB().Exec("UPDATE lifecycle SET value=value+1 WHERE id=1").Error
		})
	}()
	<-attempted
	select {
	case err := <-done:
		t.Fatalf("returned before wait: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	err = <-done
	var native sqlite3.Error
	if !errors.As(err, &native) || native.Code != sqlite3.ErrBusy || callbacks != 0 || time.Since(start) < 200*time.Millisecond || time.Since(start) > time.Second {
		t.Fatalf("busy callbacks=%d elapsed=%s err=%v", callbacks, time.Since(start), err)
	}
	t.Logf("holder acquired before contender; busy=%d callbacks=%d elapsed=%s", native.Code, callbacks, time.Since(start))
	canceled, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	err = store.InTransaction(canceled, func(resource.Tx) error { callbacks++; return nil })
	if canceled.Err() != context.DeadlineExceeded || err == nil || callbacks != 0 {
		t.Fatalf("cancel callback=%d err=%v context=%v", callbacks, err, canceled.Err())
	}
	t.Logf("cancel waiting: native=%T %v context=%v", err, err, canceled.Err())
	if err := holder.Commit().Error; err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("hook failed")
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		callbacks++
		if err := tx.DB().Exec("UPDATE lifecycle SET value=99 WHERE id=1").Error; err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := store.InTransaction(ctx, func(tx resource.Tx) error {
		callbacks++
		var value int64
		if err := tx.DB().Raw("SELECT value FROM lifecycle WHERE id=1").Scan(&value).Error; err != nil {
			return err
		}
		if value != 0 {
			t.Fatalf("partial commit %d", value)
		}
		return tx.DB().Exec("UPDATE lifecycle SET value=? WHERE id=1", value+1).Error
	}); err != nil {
		t.Fatal(err)
	}
	if callbacks != 2 {
		t.Fatalf("callback rerun: %d", callbacks)
	}
	var value int64
	if err := db.Raw("SELECT value FROM lifecycle WHERE id=1").Scan(&value).Error; err != nil || value != 1 {
		t.Fatalf("fresh request value=%d err=%v", value, err)
	}
}
