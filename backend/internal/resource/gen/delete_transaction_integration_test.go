//go:build integration

package gen

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	gormgen "gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/database"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestRepositoryDeleteScopeAndDatabaseFailures(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()
	first := seedDictType(t, a, r, "first", "delete_case_first")
	second := seedDictType(t, a, r, "second", "delete_case_second")
	scope := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", first.ID) })
	for _, id := range []int64{second.ID, -1} {
		if err := a.DeleteByKey(ctx, r, id, scope); !errors.Is(err, resource.ErrNotFound) {
			t.Fatalf("scoped delete %d: %v", id, err)
		}
		if _, err := a.BatchDelete(ctx, r, []int64{first.ID, id}, scope); !errors.Is(err, resource.ErrNotFound) {
			t.Fatalf("scoped batch: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := a.DeleteByKey(canceled, r, first.ID, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("context: %v", err)
	}
	if _, err := a.BatchDelete(canceled, r, []int64{first.ID}, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("batch context: %v", err)
	}
	suppress := true
	if database.DriverOf(r.DB()) != "postgres" {
		if err := r.DB().Callback().Delete().Before("gorm:delete").Register("test:partial_delete", func(tx *gorm.DB) {
			if suppress && tx.Statement.Table == "sys_dict_type" {
				tx.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "dict_type", Value: "delete_case_second"}}})
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := r.DB().Callback().Delete().Remove("test:partial_delete"); err != nil {
				t.Error(err)
			}
		})
	} else {
		if err := r.DB().Exec(`CREATE FUNCTION delete_case_skip() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.dict_type = 'delete_case_second' THEN RETURN NULL; END IF; RETURN OLD; END $$`).Error; err != nil {
			t.Fatal(err)
		}
		if err := r.DB().Exec(`CREATE TRIGGER delete_case_skip BEFORE DELETE ON sys_dict_type FOR EACH ROW EXECUTE FUNCTION delete_case_skip()`).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, keys := range [][]int64{{second.ID}, {first.ID, second.ID}} {
		err := r.InTransaction(ctx, func(tx *repo.Repo) error {
			_, err := a.BatchDelete(ctx, tx, keys, resource.UnrestrictedAccessScope())
			return err
		})
		if err == nil || (len(keys) == 1 && !errors.Is(err, resource.ErrNotFound)) || (len(keys) == 2 && errors.Is(err, resource.ErrNotFound)) {
			t.Fatalf("affected rows %v: %v", keys, err)
		}
	}
	if err := a.DeleteByKey(ctx, r, second.ID, resource.UnrestrictedAccessScope()); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("zero-row delete: %v", err)
	}
	suppress = false
	if database.DriverOf(r.DB()) == "postgres" {
		if err := r.DB().Exec("DROP TRIGGER delete_case_skip ON sys_dict_type").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := r.DB().Exec("CREATE TABLE delete_case_child(parent_id BIGINT, FOREIGN KEY(parent_id) REFERENCES sys_dict_type(id))").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.DB().Exec("INSERT INTO delete_case_child VALUES (?)", second.ID).Error; err != nil {
		t.Fatal(err)
	}
	err := r.InTransaction(ctx, func(tx *repo.Repo) error {
		_, err := a.BatchDelete(ctx, tx, []int64{first.ID, second.ID}, resource.UnrestrictedAccessScope())
		return err
	})
	testutil.RequireNativeError(t, r.DB(), err, "23503", 1451)
	for _, id := range []int64{first.ID, second.ID} {
		if _, err := a.GetByKey(ctx, r, id, resource.UnrestrictedAccessScope()); err != nil {
			t.Fatalf("rolled back %d: %v", id, err)
		}
	}
}

func TestDeleteModelCallbacks(t *testing.T) {
	a, r := setupRepo(t)
	if err := r.DB().Exec("CREATE TABLE delete_case_events (kind text)").Error; err != nil {
		t.Fatal(err)
	}
	for _, useGen := range []bool{false, true} {
		for _, fail := range []string{"", "before", "after"} {
			row := seedDictType(t, a, r, "callback", "delete_case_callback")
			ctx := context.WithValue(context.Background(), deleteHookContextKey{}, fail)
			err := r.DB().Transaction(func(tx *gorm.DB) error {
				db := tx.WithContext(ctx)
				if !useGen {
					return db.Where("id = ?", row.ID).Delete(new(deleteCallbackModel)).Error
				}
				var item gormgen.DO
				item.UseDB(db)
				item.UseModel(new(deleteCallbackModel))
				_, err := item.Where(field.NewInt64("sys_dict_type", "id").Eq(row.ID)).Delete()
				return err
			})
			var calls []string
			if e := r.DB().Table("delete_case_events").Pluck("kind", &calls).Error; e != nil {
				t.Fatal(e)
			}
			_, lookup := a.GetByKey(context.Background(), r, row.ID, resource.UnrestrictedAccessScope())
			if fail == "" {
				if err != nil || !errors.Is(lookup, resource.ErrNotFound) || !reflect.DeepEqual(calls, []string{"before", "after"}) {
					t.Fatalf("gen=%v: %v %v %v", useGen, err, lookup, calls)
				}
			} else if !errors.Is(err, errDeleteModelHook) || lookup != nil || len(calls) != 0 {
				t.Fatalf("gen=%v fail=%s: %v lookup=%v calls=%v", useGen, fail, err, lookup, calls)
			}
			if e := r.DB().Exec("DELETE FROM sys_dict_type WHERE id = ?", row.ID).Error; e != nil {
				t.Fatal(e)
			}
			if e := r.DB().Exec("DELETE FROM delete_case_events").Error; e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestResourceDeleteEventsAndCommit(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, fail := range []string{"", "before", "after", "commit"} {
			t.Run(fmt.Sprintf("batch=%v/%s", batch, fail), func(t *testing.T) {
				a, r := setupRepo(t)
				ctx := context.Background()
				rows := []*model.SysDictType{seedDictType(t, a, r, "one", "delete_case_one"), seedDictType(t, a, r, "two", "delete_case_two")}
				eventDDL := "CREATE TABLE delete_case_events (kind varchar(32) UNIQUE DEFERRABLE INITIALLY DEFERRED)"
				if database.DriverOf(r.DB()) == "mysql" || database.DriverOf(r.DB()) == "mariadb" {
					eventDDL = "CREATE TABLE delete_case_events (kind varchar(32) UNIQUE)"
				}
				if err := r.DB().Exec(eventDDL).Error; err != nil {
					t.Fatal(err)
				}
				sentinel := errors.New("resource hook failure")
				var order []string
				before := func(ctx context.Context, tx resource.Tx) error {
					order = append(order, "before")
					if err := tx.DB().Exec("INSERT INTO delete_case_events VALUES ('before')").Error; err != nil {
						return err
					}
					if fail == "before" {
						return sentinel
					}
					return nil
				}
				after := func(ctx context.Context, tx resource.Tx) error {
					order = append(order, "after")
					if err := tx.DB().Exec("INSERT INTO delete_case_events VALUES ('after')").Error; err != nil {
						return err
					}
					if fail == "after" {
						return sentinel
					}
					if fail == "commit" {
						if database.DriverOf(tx.DB()) == "mysql" || database.DriverOf(tx.DB()) == "mariadb" {
							return tx.DB().Rollback().Error
						}
						return tx.DB().Exec("INSERT INTO delete_case_events VALUES ('before')").Error
					}
					return nil
				}
				committed := func() {
					order = append(order, "commit")
					var count int64
					if err := r.DB().Table("delete_case_events").Count(&count).Error; err != nil {
						t.Fatal(err)
					}
					if count != 2 {
						t.Fatalf("committed events=%d", count)
					}
				}
				hooks := &resource.HookSet[model.SysDictType]{
					BeforeDelete: func(ctx context.Context, tx resource.Tx, m *model.SysDictType) error {
						if m.ID != rows[0].ID {
							t.Fatal("before snapshot")
						}
						return before(ctx, tx)
					},
					AfterDelete: func(ctx context.Context, tx resource.Tx, m *model.SysDictType) error { return after(ctx, tx) },
					AfterCommitDelete: func(ctx context.Context, m model.SysDictType) {
						if m.ID != rows[0].ID {
							t.Fatal("commit snapshot")
						}
						committed()
					},
					BeforeBatchDelete: func(ctx context.Context, tx resource.Tx, ms []*model.SysDictType) error {
						if len(ms) != 2 {
							t.Fatal("deduplicated snapshot")
						}
						return before(ctx, tx)
					},
					AfterBatchDelete: func(ctx context.Context, tx resource.Tx, ms []*model.SysDictType) error { return after(ctx, tx) },
					AfterCommitBatchDelete: func(ctx context.Context, ms []model.SysDictType) {
						if len(ms) != 2 {
							t.Fatal("commit snapshots")
						}
						committed()
					},
				}
				controller, err := NewDictTypeController(DictTypeControllerDeps{Store: testutil.Store(t, r), Hooks: hooks})
				if err != nil {
					t.Fatal(err)
				}
				if batch {
					_, err = controller.BatchDeleteDictTypes(ctx, &adminv1.BatchDeleteDictTypesRequest{Ids: []int64{rows[0].ID, rows[1].ID, rows[0].ID}})
				} else {
					_, err = controller.DeleteDictType(ctx, &adminv1.DeleteDictTypeRequest{Id: rows[0].ID})
				}
				var events int64
				if e := r.DB().Table("delete_case_events").Count(&events).Error; e != nil {
					t.Fatal(e)
				}
				want := []string{"before", "after", "commit"}
				if fail != "" {
					want = []string{"before", "after"}
					if fail == "before" {
						want = want[:1]
					}
					if err == nil || events != 0 {
						t.Fatalf("rollback err=%v events=%d", err, events)
					}
					for _, row := range rows {
						if _, e := a.GetByKey(ctx, r, row.ID, resource.UnrestrictedAccessScope()); e != nil {
							t.Fatal(e)
						}
					}
				} else {
					if err != nil || events != 2 {
						t.Fatalf("success: %v events=%d", err, events)
					}
					for i, row := range rows {
						_, e := a.GetByKey(ctx, r, row.ID, resource.UnrestrictedAccessScope())
						if batch || i == 0 {
							if !errors.Is(e, resource.ErrNotFound) {
								t.Fatalf("deleted: %v", e)
							}
						} else if e != nil {
							t.Fatal(e)
						}
					}
				}
				if !reflect.DeepEqual(order, want) {
					t.Fatalf("order=%v want=%v", order, want)
				}
			})
		}
	}
}
