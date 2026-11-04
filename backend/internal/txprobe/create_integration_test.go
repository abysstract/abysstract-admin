//go:build integration

package txprobe

import (
	"context"
	"errors"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/testdb"

	dbproduct "github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func newProbe(t *testing.T) (*testdb.Database, *resource.Store) {
	t.Helper()
	database := testdb.Open(t)
	store, err := resource.NewStore(database.DB)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return database, store
}

func TestCreateCommit(t *testing.T) {
	database, store := newProbe(t)
	var repositoryDB, hookDB *gorm.DB
	var afterCalls int
	err := Create(context.Background(), store, database, "product-commit", func(tx resource.Tx) error {
		repositoryDB = tx.DB()
		if err := InsertEvent(context.Background(), tx, database, "product-commit", "created"); err != nil {
			return err
		}
		hookDB = tx.DB()
		return nil
	}, func() {
		afterCalls++
		if countBy(t, database, "events", "event_key", "created") != 1 {
			t.Error("after-commit callback could not read committed event")
		}
	}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repositoryDB == nil || hookDB == nil || repositoryDB != hookDB {
		t.Fatalf("repository tx=%p hook tx=%p, want the same transaction handle", repositoryDB, hookDB)
	}
	if repositoryDB == store.DB() {
		t.Fatal("transaction handle must differ from ordinary Store handle")
	}
	if afterCalls != 1 {
		t.Fatalf("after-commit calls = %d, want 1", afterCalls)
	}
	if countBy(t, database, "products", "name", "product-commit") != 1 || countBy(t, database, "events", "event_key", "created") != 1 {
		t.Fatal("committed rows are missing")
	}
}

func TestCreateHookRollback(t *testing.T) {
	database, store := newProbe(t)
	hookErr := errors.New("business hook failed")
	afterCalls := 0
	err := Create(context.Background(), store, database, "product-rollback", func(tx resource.Tx) error {
		if err := InsertEvent(context.Background(), tx, database, "product-rollback", "rollback"); err != nil {
			return err
		}
		return hookErr
	}, func() { afterCalls++ }, nil)
	if !errors.Is(err, hookErr) {
		t.Fatalf("Create error = %v, want %v", err, hookErr)
	}
	if afterCalls != 0 || countBy(t, database, "products", "name", "product-rollback") != 0 || countBy(t, database, "events", "event_key", "rollback") != 0 {
		t.Fatal("hook error must roll back both tables and suppress after-commit")
	}
}

func TestCreateRepoFailure(t *testing.T) {
	database, store := newProbe(t)
	database.SeedProduct(t, "duplicate-product")
	hookCalls := 0
	afterCalls := 0
	err := Create(context.Background(), store, database, "duplicate-product", func(resource.Tx) error {
		hookCalls++
		return nil
	}, func() { afterCalls++ }, nil)
	if err == nil {
		t.Fatal("duplicate repository write must fail")
	}
	if hookCalls != 0 || afterCalls != 0 || countBy(t, database, "products", "name", "duplicate-product") != 1 {
		t.Fatalf("repository failure hook=%d after=%d rows=%d", hookCalls, afterCalls, countBy(t, database, "products", "name", "duplicate-product"))
	}
}

func TestCreateCommitFailure(t *testing.T) {
	database, store := newProbe(t)
	database.SeedEvent(t, "existing", "duplicate-event")
	afterCalls := 0
	err := Create(context.Background(), store, database, "product-commit-failure", func(tx resource.Tx) error {

		if dbproduct.DriverOf(tx.DB()) == "sqlite" {
			if err := InsertEvent(context.Background(), tx, database, "product-commit-failure", "rollback-event"); err != nil {
				return err
			}
			return tx.DB().Exec("UPDATE "+database.Event+" SET commit_guard=-1 WHERE event_key=?", "rollback-event").Error
		}
		if dbproduct.DriverOf(tx.DB()) == "mysql" || dbproduct.DriverOf(tx.DB()) == "mariadb" {
			if err := InsertEvent(context.Background(), tx, database, "product-commit-failure", "rollback-event"); err != nil {
				return err
			}
			return tx.DB().Rollback().Error
		}
		return InsertEvent(context.Background(), tx, database, "product-commit-failure", "duplicate-event")
	}, func() { afterCalls++ }, nil)
	if err == nil {
		t.Fatal("transaction commit must fail")
	}

	if dbproduct.DriverOf(database.DB) == "sqlite" {
		var native sqlite3.Error
		if !errors.As(err, &native) || native.ExtendedCode != sqlite3.ErrConstraintForeignKey {
			t.Fatalf("native deferred FK commit error: %v", err)
		}
		if countBy(t, database, "events", "event_key", "rollback-event") != 0 {
			t.Fatal("commit failure leaked event")
		}
		t.Logf("native commit error=%d; deferred event and product rolled back; AfterCommit=%d", native.ExtendedCode, afterCalls)
	}
	if afterCalls != 0 || countBy(t, database, "products", "name", "product-commit-failure") != 0 || countBy(t, database, "events", "event_key", "duplicate-event") != 1 {
		t.Fatalf("commit failure left rows or called after-commit: err=%v after=%d", err, afterCalls)
	}
}

func TestCreatePanicRollback(t *testing.T) {
	database, store := newProbe(t)
	panicValue := errors.New("business panic")
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_ = Create(context.Background(), store, database, "product-panic", func(tx resource.Tx) error {
			if err := InsertEvent(context.Background(), tx, database, "product-panic", "panic"); err != nil {
				return err
			}
			panic(panicValue)
		}, nil, nil)
	}()
	if recovered != panicValue {
		t.Fatalf("panic = %v, want original panic", recovered)
	}
	if countBy(t, database, "products", "name", "product-panic") != 0 || countBy(t, database, "events", "event_key", "panic") != 0 {
		t.Fatal("panic must roll back both tables")
	}
}

func TestCreateCanceled(t *testing.T) {
	database, store := newProbe(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Create(ctx, store, database, "product-canceled", func(tx resource.Tx) error {
		if err := InsertEvent(ctx, tx, database, "product-canceled", "canceled"); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	}, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Create error = %v, want context.Canceled", err)
	}
	if countBy(t, database, "products", "name", "product-canceled") != 0 || countBy(t, database, "events", "event_key", "canceled") != 0 {
		t.Fatal("canceled transaction must roll back both tables")
	}
}

func TestAfterCommitPanic(t *testing.T) {
	database, store := newProbe(t)
	panicValue := errors.New("after commit panic")
	var reported any
	err := Create(context.Background(), store, database, "product-after-panic", nil, func() {
		panic(panicValue)
	}, func(value any) { reported = value })
	if err != nil {
		t.Fatalf("Create = %v, want success after commit panic", err)
	}
	if reported != panicValue || countBy(t, database, "products", "name", "product-after-panic") != 1 {
		t.Fatalf("reported=%v rowCount=%d, want panic report and committed data", reported, countBy(t, database, "products", "name", "product-after-panic"))
	}
}

func TestStoreDoesNotCloseDB(t *testing.T) {
	database, store := newProbe(t)
	if err := Create(context.Background(), store, database, "product-open", nil, nil, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatalf("get SQL DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("borrowed DB was closed: %v", err)
	}
}
