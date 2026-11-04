//go:build integration

package gen

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	gormgen "gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/testutil"

	"github.com/abysstract/abysstract-admin/backend/database"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestRepositoryCreateValuesAndErrors(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()
	for _, status := range []int16{0, 2} {
		old := &model.SysDictType{DictName: "value", DictType: "old", Status: status}
		current := &model.SysDictType{DictName: "value", DictType: "gen", Status: status}
		// The removed implementation's exact GORM expression is the behavioral oracle.
		if err := r.DB().WithContext(ctx).Clauses(clause.Returning{}).Create(old).Error; err != nil {
			t.Fatal(err)
		}
		if err := a.Create(ctx, r, current); err != nil {
			t.Fatal(err)
		}
		stored, err := a.GetByKey(ctx, r, current.ID, resource.UnrestrictedAccessScope())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(current, stored) || current.ID == 0 || current.CreateTime.IsZero() || current.UpdateTime.IsZero() || current.Status != old.Status || current.Remark != old.Remark {
			t.Fatalf("old=%+v gen=%+v stored=%+v", old, current, stored)
		}
		if err := a.Create(ctx, r, &model.SysDictType{DictType: "gen"}); !errors.Is(err, resource.ErrDuplicate) {
			t.Fatalf("duplicate: %v", err)
		}
		if err := r.DB().Exec("DELETE FROM sys_dict_type").Error; err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Date(2025, 1, 2, 3, 4, 5, 123456000, time.UTC)
	explicit := &model.SysDictType{ID: 5000, DictName: "explicit", DictType: "explicit", Status: 2, Remark: "business", CreateTime: stamp, UpdateTime: stamp}
	if err := a.Create(ctx, r, explicit); err != nil {
		t.Fatal(err)
	}
	if explicit.ID != 5000 || explicit.Status != 2 || explicit.Remark != "business" || !explicit.CreateTime.Equal(stamp) || !explicit.UpdateTime.Equal(stamp) {
		t.Fatalf("explicit values: %+v", explicit)
	}
	if err := r.DB().Exec("DELETE FROM sys_dict_type").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.DB().Exec("CREATE TABLE create_case_parent (name varchar(100) PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	testutil.AddConstraint(t, r.DB(), "sys_dict_type", "create_case_fk", "FOREIGN KEY (dict_name) REFERENCES create_case_parent(name)")
	err := a.Create(ctx, r, &model.SysDictType{DictName: "missing", DictType: "fk"})
	testutil.RequireNativeError(t, r.DB(), err, "23503", 1452)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := a.Create(canceled, r, &model.SysDictType{DictType: "canceled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("context: %v", err)
	}
}

func TestCreateModelCallbacks(t *testing.T) {
	_, r := setupRepo(t)
	if err := r.DB().Exec("DELETE FROM sys_dict_type").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.DB().Exec("CREATE TABLE create_case_events (id bigint)").Error; err != nil {
		t.Fatal(err)
	}
	for _, useGen := range []bool{false, true} {
		for _, fail := range []string{"", "before", "after"} {
			m := &createCallbackModel{SysDictType: model.SysDictType{DictType: "callback"}, Fail: fail}
			err := r.DB().Transaction(func(tx *gorm.DB) error {
				db := tx.WithContext(context.Background()).Clauses(clause.Returning{})
				if !useGen {
					return db.Create(m).Error
				}
				// Resource Create uses Gen DO.Create to retain the single model pointer.
				var item gormgen.DO
				item.UseDB(db)
				item.UseModel(&createCallbackModel{})
				return item.Create(m)
			})
			want := []string{"before"}
			if fail != "before" {
				want = append(want, "after")
			}
			if !reflect.DeepEqual(m.Calls, want) {
				t.Fatalf("gen=%v fail=%q calls=%v", useGen, fail, m.Calls)
			}
			var rows, events int64
			if e := r.DB().Model(&model.SysDictType{}).Count(&rows).Error; e != nil {
				t.Fatal(e)
			}
			if e := r.DB().Table("create_case_events").Count(&events).Error; e != nil {
				t.Fatal(e)
			}
			if fail == "" {
				if err != nil || rows != 1 || events != 1 || m.DictName != "model-before" {
					t.Fatalf("success gen=%v: %v rows=%d events=%d model=%+v", useGen, err, rows, events, m)
				}
			} else if !errors.Is(err, errCreateModelHook) || rows != 0 || events != 0 {
				t.Fatalf("rollback gen=%v fail=%s: %v rows=%d events=%d", useGen, fail, err, rows, events)
			}
			for _, table := range []string{"create_case_events", "sys_dict_type"} {
				if e := r.DB().Exec("DELETE FROM " + table).Error; e != nil {
					t.Fatal(e)
				}
			}
		}
	}
}

func TestResourceCreateLifecycle(t *testing.T) {
	_, r := setupRepo(t)
	if err := r.DB().Exec("DELETE FROM sys_dict_type").Error; err != nil {
		t.Fatal(err)
	}
	eventDDL := "CREATE TABLE create_case_events (id bigint UNIQUE DEFERRABLE INITIALLY DEFERRED)"
	if database.DriverOf(r.DB()) == "mysql" || database.DriverOf(r.DB()) == "mariadb" {
		eventDDL = "CREATE TABLE create_case_events (id bigint UNIQUE)"
	}
	if err := r.DB().Exec(eventDDL).Error; err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"success", "before", "after", "commit"} {
		t.Run(phase, func(t *testing.T) {
			calls := []string{}
			var pointer *model.SysDictType
			var snapshot model.SysDictType
			sentinel := errors.New("resource hook failure")
			store, err := resource.NewStore(r.DB())
			if err != nil {
				t.Fatal(err)
			}
			c, err := NewDictTypeController(DictTypeControllerDeps{Store: store, Hooks: &resource.HookSet[model.SysDictType]{
				BeforeCreate: func(_ context.Context, tx resource.Tx, m *model.SysDictType) error {
					calls = append(calls, "before")
					pointer = m
					m.DictName = "normalized"
					if tx.DB() == r.DB() {
						t.Fatal("hook requires transaction")
					}
					if phase == "before" {
						return sentinel
					}
					return nil
				},
				AfterCreate: func(_ context.Context, tx resource.Tx, m *model.SysDictType) error {
					calls = append(calls, "after")
					if m != pointer || m.ID == 0 || m.Status != 1 || m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
						t.Fatalf("AfterCreate snapshot: %+v", m)
					}
					snapshot = *m
					if err := tx.DB().Exec("INSERT INTO create_case_events VALUES (?)", m.ID).Error; err != nil {
						return err
					}
					if phase == "commit" {
						if database.DriverOf(tx.DB()) == "mysql" || database.DriverOf(tx.DB()) == "mariadb" {
							return tx.DB().Rollback().Error
						}
						return tx.DB().Exec("INSERT INTO create_case_events VALUES (?)", m.ID).Error
					}
					if phase == "after" {
						return sentinel
					}
					return nil
				},
				AfterCommitCreate: func(_ context.Context, m model.SysDictType) {
					calls = append(calls, "commit")
					if !reflect.DeepEqual(m, snapshot) {
						t.Fatal("AfterCommit snapshot differs")
					}
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			response, err := c.CreateDictType(context.Background(), &adminv1.CreateDictTypeRequest{DictName: "requested", DictType: phase})
			var rows, events int64
			if e := r.DB().Model(&model.SysDictType{}).Count(&rows).Error; e != nil {
				t.Fatal(e)
			}
			if e := r.DB().Table("create_case_events").Count(&events).Error; e != nil {
				t.Fatal(e)
			}
			want := []string{"before"}
			if phase != "before" {
				want = append(want, "after")
			}
			if phase == "success" {
				want = append(want, "commit")
				if err != nil || rows != 1 || events != 1 {
					t.Fatalf("success: %v rows=%d events=%d", err, rows, events)
				}
				entity, e := DictTypeToEntity(&snapshot)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(response.Item, entity) {
					t.Fatalf("response=%+v snapshot=%+v", response, entity)
				}
			} else if connect.CodeOf(err) != connect.CodeInternal || rows != 0 || events != 0 {
				t.Fatalf("rollback: %v rows=%d events=%d", err, rows, events)
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want=%v", calls, want)
			}
			for _, table := range []string{"create_case_events", "sys_dict_type"} {
				if e := r.DB().Exec("DELETE FROM " + table).Error; e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
