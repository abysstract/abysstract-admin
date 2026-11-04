//go:build integration

package gen

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestRepositoryReadScopeAndTransaction(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()
	first := seedDictType(t, a, r, "first", "read_scope_first")
	second := seedDictType(t, a, r, "second", "read_scope_second")
	scope := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", first.ID) })
	for _, read := range []struct {
		name string
		get  func(context.Context, resource.Session, int64, resource.AccessScope) (*model.SysDictType, error)
	}{{"detail", a.GetByKey}, {"locked", a.GetByKeyForUpdate}} {
		t.Run(read.name, func(t *testing.T) {
			if err := r.InTransaction(ctx, func(tx *repo.Repo) error {
				row, err := read.get(ctx, tx, first.ID, scope)
				if err != nil {
					return err
				}
				if row.ID != first.ID {
					t.Fatalf("accessible: %+v", row)
				}
				for _, key := range []int64{second.ID, -1} {
					if _, err := read.get(ctx, tx, key, scope); !errors.Is(err, resource.ErrNotFound) {
						t.Fatalf("key %d: %v", key, err)
					}
				}
				if _, err := read.get(ctx, tx, first.ID, resource.AccessScope{}); !errors.Is(err, resource.ErrNotFound) {
					t.Fatalf("deny all: %v", err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := r.InTransaction(ctx, func(tx *repo.Repo) error {
		rows, err := a.GetManyForUpdate(ctx, tx, []int64{first.ID, first.ID, second.ID, -1}, scope)
		if err != nil || len(rows) != 1 || rows[0].ID != first.ID {
			t.Fatalf("scoped batch: %+v %v", rows, err)
		}
		for _, keys := range [][]int64{nil, {}, {first.ID, second.ID}} {
			rows, err := a.GetManyForUpdate(ctx, tx, keys, resource.AccessScope{})
			if err != nil || len(rows) != 0 {
				t.Fatalf("deny batch %v: %+v %v", keys, rows, err)
			}
		}
		rows, err = a.GetManyForUpdate(ctx, tx, nil, resource.UnrestrictedAccessScope())
		if err != nil || len(rows) != 0 {
			t.Fatalf("empty batch: %+v %v", rows, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("rollback transaction-local row")
	row := &model.SysDictType{DictName: "local", DictType: "read_scope_local", Status: 1}
	if err := r.InTransaction(ctx, func(tx *repo.Repo) error {
		if err := a.Create(ctx, tx, row); err != nil {
			return err
		}
		if _, err := a.GetByKey(ctx, tx, row.ID, resource.UnrestrictedAccessScope()); err != nil {
			return err
		}
		if _, err := a.GetByKeyForUpdate(ctx, tx, row.ID, resource.UnrestrictedAccessScope()); err != nil {
			return err
		}
		rows, err := a.GetManyForUpdate(ctx, tx, []int64{row.ID}, resource.UnrestrictedAccessScope())
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			t.Fatalf("transaction-local batch: %+v", rows)
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := a.GetByKey(ctx, r, row.ID, resource.UnrestrictedAccessScope()); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("rolled-back row: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.GetByKey(canceled, r, first.ID, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("context: %v", err)
	}
	if _, err := a.GetByKeyForUpdate(canceled, r, first.ID, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("lock context: %v", err)
	}
	if _, err := a.GetManyForUpdate(canceled, r, []int64{first.ID}, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("batch context: %v", err)
	}
}

// A separate PostgreSQL connection probes the held row using NOWAIT. Rollback
// must restore both the row and its availability to another transaction.
func TestRepositoryCallerTransactionHoldsLocksUntilRollback(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()
	for _, operation := range []string{"GetByKeyForUpdate", "GetManyForUpdate", "Update", "DeleteByKey", "BatchDelete"} {
		t.Run(operation, func(t *testing.T) {
			row := seedDictType(t, a, r, operation, "read_scope_"+operation)
			other := seedDictType(t, a, r, operation+" other", "read_scope_other_"+operation)
			holder := r.DB().Begin()
			if holder.Error != nil {
				t.Fatal(holder.Error)
			}
			defer holder.Rollback()
			tx := r.WithDB(holder)
			restricted := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", row.ID) })
			switch operation {
			case "GetByKeyForUpdate":
				if _, err := a.GetByKeyForUpdate(ctx, tx, row.ID, restricted); err != nil {
					t.Fatal(err)
				}
			case "GetManyForUpdate":
				rows, err := a.GetManyForUpdate(ctx, tx, []int64{row.ID, other.ID}, restricted)
				if err != nil || len(rows) != 1 {
					t.Fatalf("scoped lock: %+v %v", rows, err)
				}
			case "Update":
				row.DictName = "changed"
				if err := a.Update(ctx, tx, row.ID, row, resource.NewColumnSet("dict_name"), restricted); err != nil {
					t.Fatal(err)
				}
			case "DeleteByKey":
				if err := a.DeleteByKey(ctx, tx, row.ID, restricted); err != nil {
					t.Fatal(err)
				}
			case "BatchDelete":
				if _, err := a.BatchDelete(ctx, tx, []int64{row.ID}, restricted); err != nil {
					t.Fatal(err)
				}
			}
			if database.DriverOf(r.DB()) == "sqlite" {
				start := time.Now()
				contender := r.DB().Begin()
				var native sqlite3.Error
				if !errors.As(contender.Error, &native) || native.Code != sqlite3.ErrBusy || time.Since(start) < 200*time.Millisecond {
					t.Fatalf("BEGIN IMMEDIATE waits before callback: %v", contender.Error)
				}
				t.Logf("%s: reserved writer blocks competing BEGIN for %s", operation, time.Since(start))
				if err := holder.Rollback().Error; err != nil {
					t.Fatal(err)
				}
			} else {

				contender := r.DB().Begin()
				if contender.Error != nil {
					t.Fatal(contender.Error)
				}
				defer contender.Rollback()
				if err := contender.Exec("SELECT id FROM sys_dict_type WHERE id = ? FOR UPDATE NOWAIT", other.ID).Error; err != nil {
					t.Fatalf("outside scope locked: %v", err)
				}
				err := contender.Exec("SELECT id FROM sys_dict_type WHERE id = ? FOR UPDATE NOWAIT", row.ID).Error
				code := uint16(3572)
				if database.DriverOf(r.DB()) == "mariadb" {
					code = 1205
				}
				testutil.RequireNativeError(t, r.DB(), err, "55P03", code)
				if err := contender.Rollback().Error; err != nil {
					t.Fatal(err)
				}
				if err := holder.Rollback().Error; err != nil {
					t.Fatal(err)
				}
			}
			probe := r.DB().Begin()
			if probe.Error != nil {
				t.Fatal(probe.Error)
			}
			defer probe.Rollback()
			var restored model.SysDictType
			query := "SELECT * FROM sys_dict_type WHERE id = ?"
			if database.DriverOf(r.DB()) != "sqlite" {
				query += " FOR UPDATE NOWAIT"
			}
			if err := probe.Raw(query, row.ID).Scan(&restored).Error; err != nil {
				t.Fatalf("lock after rollback: %v", err)
			}
			if restored.ID != row.ID || restored.DictName != operation {
				t.Fatalf("rollback row: %+v", restored)
			}
			if err := probe.Rollback().Error; err != nil {
				t.Fatal(err)
			}
		})
	}
}
