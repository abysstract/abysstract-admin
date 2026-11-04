//go:build integration

package gen_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	gen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestDefaultRepositoryLists(t *testing.T) {
	t.Run("Config", func(t *testing.T) {
		checkRepositoryList[model.SysConfig](t, "sys_config", "config_name", map[string]any{"config_key": "listcase"},
			gen.NewConfigRepository().List, func(page int, keyword string) gen.ConfigListQuery {
				return gen.ConfigListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.ConfigListQuery{Keyword: "ListCase", PageSize: 10}, 3)
	})
	t.Run("Dept", func(t *testing.T) {
		checkRepositoryList[model.SysDept](t, "sys_dept", "dept_name", map[string]any{},
			gen.NewDeptRepository().List, func(page int, keyword string) gen.DeptListQuery {
				return gen.DeptListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.DeptListQuery{Keyword: "ListCase", PageSize: 10, Status: ptr(int16(0))}, 1)
	})
	t.Run("DictData", func(t *testing.T) {
		checkRepositoryList[model.SysDictDatum](t, "sys_dict_data", "dict_label", map[string]any{"dict_type": "listcase", "dict_value": "v"},
			gen.NewDictDataRepository().List, func(page int, keyword string) gen.DictDataListQuery {
				return gen.DictDataListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.DictDataListQuery{Keyword: "ListCase", PageSize: 10, DictType: ptr("listcase1")}, 1)
	})
	t.Run("DictType", func(t *testing.T) {
		checkRepositoryList[model.SysDictType](t, "sys_dict_type", "dict_name", map[string]any{"dict_type": "listcase"},
			gen.NewDictTypeRepository().List, func(page int, keyword string) gen.DictTypeListQuery {
				return gen.DictTypeListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.DictTypeListQuery{Keyword: "ListCase", PageSize: 10, DictType: ptr("listcase1")}, 1)
	})
	t.Run("Menu", func(t *testing.T) {
		checkRepositoryList[model.SysMenu](t, "sys_menu", "menu_name", map[string]any{},
			gen.NewMenuRepository().List, func(page int, keyword string) gen.MenuListQuery {
				return gen.MenuListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.MenuListQuery{Keyword: "ListCase", PageSize: 10, Status: ptr(int16(0))}, 1)
	})
	t.Run("Notice", func(t *testing.T) {
		checkRepositoryList[model.SysNotice](t, "sys_notice", "notice_title", map[string]any{},
			gen.NewNoticeRepository().List, func(page int, keyword string) gen.NoticeListQuery {
				return gen.NoticeListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.NoticeListQuery{Keyword: "ListCase", PageSize: 10}, 3)
	})
	t.Run("Role", func(t *testing.T) {
		checkRepositoryList[model.SysRole](t, "sys_role", "role_name", map[string]any{"role_key": "listcase"},
			gen.NewRoleRepository().List, func(page int, keyword string) gen.RoleListQuery {
				return gen.RoleListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.RoleListQuery{Keyword: "ListCase", PageSize: 10, Status: ptr(int16(0))}, 1)
	})
	t.Run("User", func(t *testing.T) {
		checkRepositoryList[model.SysUser](t, "sys_user", "username", map[string]any{},
			gen.NewUserRepository().List, func(page int, keyword string) gen.UserListQuery {
				return gen.UserListQuery{PageNum: page, PageSize: 1, Keyword: keyword, Orders: []resource.Order{{Column: "create_time"}}}
			},
			gen.UserListQuery{Keyword: "ListCase", PageSize: 10, DeptId: ptr(int64(0))}, 1)
	})
}

// Run the same access, pagination, transaction and error contract through every
// generated default adapter, including the four overridden in production.
func checkRepositoryList[M, Q any](t *testing.T, table, nameColumn string, extra map[string]any,
	list func(context.Context, resource.Session, Q, resource.AccessScope) ([]*M, int64, error),
	query func(int, string) Q, filtered Q, filteredCount int64) {
	t.Helper()
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	scope := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id >= ?", 9001) })
	names := []string{`ListCase%_\alpha`, `ListCase%_\beta`, "ListCasexydecoy"}
	seed := func(handle *gorm.DB, index int) error {
		row := map[string]any{"id": 9000 + index, nameColumn: names[(index-1)%3], "create_time": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
		for k, v := range extra {
			row[k] = fmt.Sprint(v, index)
		}
		if table == "sys_user" {
			row[nameColumn] = fmt.Sprint(row[nameColumn], index)
			row["dept_id"] = int64(1)
		}
		if table == "sys_user" && index == 1 {
			row["dept_id"] = int64(0)
		}
		if table == "sys_dept" || table == "sys_menu" || table == "sys_role" {
			if index == 1 {
				row["status"] = int16(0)
			}
		}
		if (table == "sys_dept" || table == "sys_role" || table == "sys_user") && index == 3 {
			row["del_flag"] = int16(1)
		}
		return handle.Table(table).Create(row).Error
	}
	for i := 1; i <= 3; i++ {
		if err := seed(db, i); err != nil {
			t.Fatal(err)
		}
	}
	for _, word := range []string{"ListCase", "%", "_", `\`, "  listcase%  "} {
		wantTotal := int64(2)
		if word == "ListCase" {
			wantTotal = 3
		}
		for page := 1; page <= int(wantTotal)+1; page++ {
			rows, total, err := list(ctx, store, query(page, word), scope)
			wantLen := 1
			if page > int(wantTotal) {
				wantLen = 0
			}
			if err != nil || total != wantTotal || len(rows) != wantLen {
				t.Fatalf("word=%q page=%d rows=%v total=%d err=%v", word, page, rows, total, err)
			}
			if len(rows) > 0 && reflect.ValueOf(rows[0]).Elem().FieldByName("ID").Int() != int64(9000+page) {
				t.Fatalf("unstable order: %+v", rows[0])
			}
		}
	}
	rows, total, err := list(ctx, store, filtered, scope)
	if err != nil || total != filteredCount || len(rows) != int(filteredCount) {
		t.Fatalf("typed filter: rows=%v total=%d err=%v", rows, total, err)
	}
	for i, access := range []resource.AccessScope{{}, resource.NewAccessScope(nil), resource.NewAccessScope(func(*gorm.DB) *gorm.DB { return nil }), resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", 9002) })} {
		rows, total, err := list(ctx, store, query(1, "ListCase"), access)
		if err != nil {
			t.Fatal(err)
		}

		want := int64(0)
		if i == 3 {
			want = 1
		}
		if total != want || len(rows) != int(want) {
			t.Fatalf("scope %d: total=%d rows=%v", i, total, rows)
		}
		if i == 3 && reflect.ValueOf(rows[0]).Elem().FieldByName("ID").Int() != 9002 {
			t.Fatal("restricted scope row")
		}

	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := list(cancelled, store, query(1, "ListCase"), scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("context: %v", err)
	}
	rollback := errors.New("stage I rollback")
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		if err := seed(tx.DB(), 4); err != nil {
			return err
		}
		_, inside, err := list(ctx, tx, query(1, "ListCase"), scope)
		if err != nil {
			return err
		}
		_, outside, err := list(ctx, store, query(1, "ListCase"), scope)
		if err != nil {
			return err
		}
		if inside != 4 || outside != 3 {
			t.Fatalf("transaction visibility inside=%d outside=%d", inside, outside)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	_, total, err = list(ctx, store, query(1, "ListCase"), scope)
	if err != nil || total != 3 {
		t.Fatalf("rollback total=%d err=%v", total, err)
	}
	injected := errors.New("stage I database error")
	for _, failFind := range []bool{false, true} {
		if err := db.Callback().Query().Before("gorm:query").Register("listcaseerror", func(tx *gorm.DB) {
			_, isCount := tx.Statement.Dest.(*int64)
			if isCount != failFind {
				tx.AddError(injected)
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, _, err := list(ctx, store, query(1, "ListCase"), scope)
		if removeErr := db.Callback().Query().Remove("listcaseerror"); removeErr != nil {
			t.Fatal(removeErr)
		}
		if !errors.Is(err, injected) {
			t.Fatalf("database failure find=%v: %v", failFind, err)
		}
	}
	if err := db.Migrator().RenameTable(table, table+"_unavailable"); err != nil {
		t.Fatal(err)
	}
	_, _, err = list(ctx, store, query(1, "ListCase"), scope)
	testutil.RequireNativeError(t, db, err, "42P01", 1146)

}
