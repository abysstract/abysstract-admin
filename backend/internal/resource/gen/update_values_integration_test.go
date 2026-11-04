//go:build integration

package gen_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/database"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	gen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestDefaultRepositorySelectedUpdates(t *testing.T) {
	t.Run("Config", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_config", "config_value", "ConfigValue", "ConfigName", map[string]any{"config_name": "keep", "config_key": "updatecase", "config_value": "keep"}, gen.NewConfigRepository().Update)
	})
	t.Run("Dept", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_dept", "order_num", "OrderNum", "DeptName", map[string]any{"dept_name": "keep", "order_num": 7}, gen.NewDeptRepository().Update)
	})
	t.Run("DictData", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_dict_data", "is_default", "IsDefault", "DictLabel", map[string]any{"dict_label": "keep", "dict_type": "updatecase", "dict_value": "updatecase", "is_default": 1}, gen.NewDictDataRepository().Update)
	})
	t.Run("DictType", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_dict_type", "remark", "Remark", "DictName", map[string]any{"dict_name": "keep", "dict_type": "updatecase", "remark": "keep"}, gen.NewDictTypeRepository().Update)
	})
	t.Run("Menu", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_menu", "visible", "Visible", "MenuName", map[string]any{"menu_name": "keep", "visible": 1}, gen.NewMenuRepository().Update)
	})
	t.Run("Notice", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_notice", "notice_content", "NoticeContent", "NoticeTitle", map[string]any{"notice_title": "keep", "notice_content": "keep"}, gen.NewNoticeRepository().Update)
	})
	t.Run("Role", func(t *testing.T) {
		checkSelectedUpdate(t, "sys_role", "role_sort", "RoleSort", "RoleName", map[string]any{"role_name": "keep", "role_key": "updatecase", "role_sort": 7}, gen.NewRoleRepository().Update)
	})
}

// Every generated default writer must preserve the same selection, scope and
// transaction contract, including defaults overridden by production repositories.
func checkSelectedUpdate[M any](t *testing.T, table, column, field, omitted string, seed map[string]any,
	update func(context.Context, resource.Session, int64, *M, resource.ColumnSet, resource.AccessScope) error) {
	t.Helper()
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	capture := &updateSQLCapture{Interface: logger.Default}
	db = db.Session(&gorm.Session{Logger: capture})
	seed["id"] = int64(9902)
	if err := db.Table(table).Create(seed).Error; err != nil {
		t.Fatal(err)
	}
	var original M
	if err := db.First(&original, 9902).Error; err != nil {
		t.Fatal(err)
	}
	originalValue := reflect.ValueOf(&original).Elem()
	if originalValue.FieldByName(field).IsZero() {
		t.Fatal("fixture requires a nonzero selected value")
	}
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	all := resource.UnrestrictedAccessScope()
	cols := resource.NewColumnSet(resource.Column(column))
	changed := original
	v := reflect.ValueOf(&changed).Elem()
	v.FieldByName(field).SetZero()
	v.FieldByName(omitted).SetString("unselected")
	// A future nanosecond value exercises monotonic advancement and PostgreSQL precision.
	future := time.Now().Add(time.Hour)
	v.FieldByName("UpdateTime").Set(reflect.ValueOf(future))
	rollback := errors.New("rollback")
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		if err := update(ctx, tx, 9902, &changed, cols, all); err != nil {
			return err
		}
		var inside, outside M
		if err := tx.DB().First(&inside, 9902).Error; err != nil {
			return err
		}
		if err := db.First(&outside, 9902).Error; err != nil {
			return err
		}
		iv := reflect.ValueOf(&inside).Elem()
		audit := iv.FieldByName("UpdateTime").Interface().(time.Time)
		want := resource.NormalizeTimestamp(future).Add(time.Microsecond)
		if !iv.FieldByName(field).IsZero() || iv.FieldByName(omitted).Interface() != originalValue.FieldByName(omitted).Interface() || !audit.Equal(want) || !audit.Equal(v.FieldByName("UpdateTime").Interface().(time.Time)) || !reflect.DeepEqual(outside, original) {
			t.Fatalf("selection/time/transaction: inside=%+v changed=%+v outside=%+v", inside, changed, outside)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var saved M
	if err := db.First(&saved, 9902).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, original) {
		t.Fatal("rollback changed row")
	}
	updates := 0
	locked := false
	for _, sql := range capture.statements {
		if strings.Contains(sql, "FOR UPDATE") {
			locked = true
		}
		if strings.HasPrefix(sql, "UPDATE ") {
			updates++
			if strings.Count(strings.ReplaceAll(sql, "`", `"`), `"update_time"=`) != 1 || strings.Contains(strings.ReplaceAll(sql, "`", `"`), `"create_time"=`) {
				t.Fatalf("audit SET: %s", sql)
			}
		}
	}
	if updates != 1 || locked != (database.DriverOf(db) != "sqlite") {
		t.Fatalf("update count=%d locked=%v", updates, locked)
	}
	for _, access := range []resource.AccessScope{{}, resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", -1) })} {
		if err := update(ctx, store, 9902, &changed, cols, access); !errors.Is(err, resource.ErrNotFound) {
			t.Fatalf("read scope: %v", err)
		}
	}
	calls := 0
	writeDenied := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB {
		calls++
		if calls == 1 {
			return db
		}
		return db.Where("id = ?", -1)
	})
	if err := update(ctx, store, 9902, &changed, cols, writeDenied); !errors.Is(err, resource.ErrNotFound) || calls != 3 {
		t.Fatalf("write scope: calls=%d err=%v", calls, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := update(canceled, store, 9902, &changed, cols, all); !errors.Is(err, context.Canceled) {
		t.Fatalf("context: %v", err)
	}
	if err := update(ctx, store, -1, &changed, cols, all); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	for _, invalid := range []resource.ColumnSet{{}, resource.NewColumnSet(resource.Column(column), resource.Column(column)), resource.NewColumnSet("id"), resource.NewColumnSet("update_time"), resource.NewColumnSet("unknown")} {
		if err := update(ctx, store, 9902, &changed, invalid, all); err == nil {
			t.Fatalf("accepted invalid columns %v", invalid.Values())
		}
	}
	// Gen's column update skips model hooks while executing the SQL callback chain.
	injected := errors.New("write failure")
	const callback = "updatecase:update-error"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if !tx.Statement.SkipHooks {
			t.Error("column update must skip model hooks")
		}
		tx.AddError(injected)
	}); err != nil {
		t.Fatal(err)
	}
	err = update(ctx, store, 9902, &changed, cols, all)
	if removeErr := db.Callback().Update().Remove(callback); removeErr != nil {
		t.Fatal(removeErr)
	}
	if !errors.Is(err, injected) || errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("database error: %v", err)
	}
	if err := db.First(&saved, 9902).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, original) {
		t.Fatal("failed writes changed row")
	}
}
