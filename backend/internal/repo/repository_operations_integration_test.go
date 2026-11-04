//go:build integration

package repo

import (
	"bytes"
	"context"
	"errors"
	"log"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func newRepositoryFixture(t *testing.T) *Repo {
	t.Helper()
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	execFixtureSQL(t, db, `
 INSERT INTO sys_user(id,username,password) VALUES (8101,'p1-one','old-hash'),(8102,'p1-two','old-hash'),(8103,'p1-three','old-hash');
 INSERT INTO sys_role(id,role_name,role_key) VALUES (8101,'p1-one','p1-one'),(8102,'p1-two','p1-two'),(8103,'p1-three','p1-three');
 INSERT INTO sys_menu(id,menu_name,menu_type) VALUES (8101,'p1-one','C'),(8102,'p1-two','C'),(8103,'p1-three','C');
 INSERT INTO sys_user_role(user_id,role_id) VALUES (8101,8101),(8102,8102),(8103,8103);
 INSERT INTO sys_role_menu(role_id,menu_id) VALUES (8101,8101),(8101,8102),(8102,8102),(8103,8103);
 INSERT INTO sys_role_dept(role_id,dept_id) VALUES (8101,1),(8102,2),(8103,3);`)
	return New(db)
}

func TestRelationDeletes(t *testing.T) {
	ctx := context.Background()
	all := UnrestrictedAccessScope()
	for _, kind := range []string{"user", "role", "menu"} {
		for _, batch := range []bool{false, true} {
			name := kind + "/single"
			if batch {
				name = kind + "/batch"
			}
			t.Run(name, func(t *testing.T) {
				r := newRepositoryFixture(t)
				ids := []int64{8101}
				if batch {
					ids = append(ids, 8102)
				}
				if got, err := r.RoleIDsOf(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8101}) {
					t.Fatalf("roles=%v err=%v", got, err)
				}
				if got, err := r.MenuIDsOfRole(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8101, 8102}) {
					t.Fatalf("menus=%v err=%v", got, err)
				}
				err := r.InTransaction(ctx, func(tx *Repo) error {
					switch kind {
					case "user":
						if batch {
							_, e := tx.BatchDeleteUsers(ctx, ids)
							return e
						}
						return tx.DeleteUser(ctx, ids[0])
					case "role":
						if batch {
							_, e := tx.BatchDeleteRoleResources(ctx, ids, all)
							return e
						}
						return tx.DeleteRoleResource(ctx, ids[0], all)
					default:
						if batch {
							_, e := tx.BatchDeleteMenuResources(ctx, ids, all)
							return e
						}
						return tx.DeleteMenuResource(ctx, ids[0], all)
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				tables := []string{"sys_user_role"}
				col := "user_id"
				if kind == "role" {
					tables = []string{"sys_user_role", "sys_role_menu", "sys_role_dept"}
					col = "role_id"
				}
				if kind == "menu" {
					tables = []string{"sys_role_menu"}
					col = "menu_id"
				}
				for _, table := range tables {
					if n := countFixtureRows(t, r.db, table, col+" IN ?", ids); n != 0 {
						t.Fatalf("%s deleted relations=%d", table, n)
					}
					if n := countFixtureRows(t, r.db, table, col+" = ?", 8103); n != 1 {
						t.Fatalf("%s unrelated=%d", table, n)
					}
				}
				if _, err := r.RoleIDsOf(ctx, -1); err != nil {
					t.Fatal(err)
				}
				if _, err := r.MenuIDsOfRole(ctx, -1); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestRelationDeleteFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"user", "role"} {
		for _, batch := range []bool{false, true} {
			name := kind + "/single"
			if batch {
				name = kind + "/batch"
			}
			t.Run(name, func(t *testing.T) {
				r := newRepositoryFixture(t)
				// A real FK prevents relation deletion after the soft-delete has executed.
				execFixtureSQL(t, r.db, `CREATE TABLE p1_relation_guard(user_id BIGINT,role_id BIGINT, FOREIGN KEY(user_id,role_id) REFERENCES sys_user_role(user_id,role_id)); INSERT INTO p1_relation_guard VALUES(8101,8101)`)
				err := r.InTransaction(ctx, func(tx *Repo) error {
					if kind == "user" {
						if batch {
							_, e := tx.BatchDeleteUsers(ctx, []int64{8101, 8102})
							return e
						}
						return tx.DeleteUser(ctx, 8101)
					}
					if batch {
						_, e := tx.BatchDeleteRoleResources(ctx, []int64{8101, 8102}, UnrestrictedAccessScope())
						return e
					}
					return tx.DeleteRoleResource(ctx, 8101, UnrestrictedAccessScope())
				})
				if err == nil {
					t.Fatal("relation deletion succeeded despite FK")
				}
				table := "sys_user"
				if kind == "role" {
					table = "sys_role"
				}
				if n := countFixtureRows(t, r.db, table, "id IN ? AND del_flag = 0", []int64{8101, 8102}); n != 2 {
					t.Fatalf("active=%d", n)
				}
				if n := countFixtureRows(t, r.db, "sys_user_role", "user_id IN ?", []int64{8101, 8102}); n != 2 {
					t.Fatalf("relations=%d", n)
				}
			})
		}
	}
}

func TestRelationCallerRollbackAndEmpty(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	all := UnrestrictedAccessScope()
	rollback := errors.New("caller rollback")
	err := r.InTransaction(ctx, func(tx *Repo) error {
		if err := tx.DeleteMenuResource(ctx, 8101, all); err != nil {
			return err
		}
		if err := tx.DeleteRoleResource(ctx, 8102, all); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_role_menu", "role_id IN ?", []int64{8101, 8102}); n != 3 {
		t.Fatalf("restored relations=%d", n)
	}
	for _, table := range []string{"sys_user_role", "sys_role_menu", "sys_role_dept"} {
		if n := countFixtureRows(t, r.db, table, "role_id = ?", 8102); n != 1 {
			t.Fatalf("%s restored relations=%d", table, n)
		}
	}
	if n := countFixtureRows(t, r.db, "sys_role", "id=8102 AND del_flag=0"); n != 1 {
		t.Fatalf("restored role=%d", n)
	}
	if _, err := r.BatchDeleteRoleResources(ctx, nil, all); err == nil {
		t.Fatal("empty roles accepted")
	}
	if _, err := r.BatchDeleteMenuResources(ctx, nil, all); err == nil {
		t.Fatal("empty menus accepted")
	}
	if n, err := r.BatchDeleteUsers(ctx, nil); err != nil || n != 0 {
		t.Fatalf("empty users=%d %v", n, err)
	}
	var u model.SysUser
	if err := r.db.First(&u, 8102).Error; err != nil {
		t.Fatal(err)
	}
	if u.DelFlag != 0 {
		t.Fatal("user changed")
	}
	// Existing records with empty relation sets remain deletable.
	execFixtureSQL(t, r.db, `DELETE FROM sys_user_role WHERE user_id=8102 OR role_id=8101;
 DELETE FROM sys_role_menu WHERE role_id=8101 OR menu_id=8103;
 DELETE FROM sys_role_dept WHERE role_id=8101`)
	if err := r.DeleteUser(ctx, 8102); err != nil {
		t.Fatal(err)
	}
	if err := r.InTransaction(ctx, func(tx *Repo) error {
		if err := tx.DeleteRoleResource(ctx, 8101, all); err != nil {
			return err
		}
		return tx.DeleteMenuResource(ctx, 8103, all)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceBindings(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	all := UnrestrictedAccessScope()
	if err := r.InTransaction(ctx, func(tx *Repo) error { return replaceUserRoles(tx.db, 8101, []int64{8102, 8102, 8103}) }); err != nil {
		t.Fatal(err)
	}
	if got, err := r.RoleIDsOf(ctx, 8101); err != nil || !intSliceEqualSorted(got, []int64{8102, 8103}) {
		t.Fatalf("roles=%v %v", got, err)
	}
	if err := r.InTransaction(ctx, func(tx *Repo) error { return replaceUserRoles(tx.db, 8101, nil) }); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_user_role", "user_id = ?", 8101); n != 0 {
		t.Fatalf("empty replacement=%d", n)
	}
	execFixtureSQL(t, r.db, `INSERT INTO sys_user_role VALUES(8101,8101)`)
	testutil.AddConstraint(t, r.db, "sys_user_role", "p1_reject_role", "CHECK(role_id <> 8199)")
	err := r.InTransaction(ctx, func(tx *Repo) error { return replaceUserRoles(tx.db, 8101, []int64{8102, 8199}) })
	if err == nil {
		t.Fatal("constraint failure missing")
	}
	if got, err := r.RoleIDsOf(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8101}) {
		t.Fatalf("old roles=%v %v", got, err)
	}
	if err := r.InTransaction(ctx, func(tx *Repo) error { return tx.UpdateRoleMenusResource(ctx, 8101, []int64{8103, 8103, 8102}, all) }); err != nil {
		t.Fatal(err)
	}
	if got, err := r.MenuIDsOfRole(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8102, 8103}) {
		t.Fatalf("menus=%v %v", got, err)
	}
	err = r.InTransaction(ctx, func(tx *Repo) error {
		err := tx.UpdateRoleMenusResource(ctx, 8101, []int64{-1}, all)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing menu=%v", err)
		}
		// Inspect before the caller rolls back: validation must precede deletion.
		if got, readErr := tx.MenuIDsOfRole(ctx, 8101); readErr != nil || !reflect.DeepEqual(got, []int64{8102, 8103}) {
			t.Fatalf("menus before caller rollback=%v %v", got, readErr)
		}
		return err
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing menu=%v", err)
	}
	if got, err := r.MenuIDsOfRole(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8102, 8103}) {
		t.Fatalf("menus after validation failure=%v %v", got, err)
	}
	testutil.AddConstraint(t, r.db, "sys_role_menu", "p1_reject_menu", "CHECK(role_id <> 8101 OR menu_id <> 8101)")
	err = r.InTransaction(ctx, func(tx *Repo) error { return tx.UpdateRoleMenusResource(ctx, 8101, []int64{8102, 8101}, all) })
	if err == nil {
		t.Fatal("menu constraint failure missing")
	}
	if got, err := r.MenuIDsOfRole(ctx, 8101); err != nil || !reflect.DeepEqual(got, []int64{8102, 8103}) {
		t.Fatalf("restored menus=%v %v", got, err)
	}
	if err := r.InTransaction(ctx, func(tx *Repo) error { return tx.UpdateRoleMenusResource(ctx, 8101, nil, all) }); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_role_menu", "role_id = ?", 8101); n != 0 {
		t.Fatalf("cleared menus=%d", n)
	}
	if n := countFixtureRows(t, r.db, "sys_role_menu", "role_id = ?", 8102); n != 1 {
		t.Fatalf("other menus=%d", n)
	}
}

func TestAuthorizationJoins(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	execFixtureSQL(t, r.db, `DELETE FROM sys_role_menu WHERE role_id BETWEEN 8101 AND 8104;
 INSERT INTO sys_role(id,role_name,role_key,del_flag) VALUES(8104,'deleted','p1-deleted',1);
 UPDATE sys_role SET status=2 WHERE id=8103;
 INSERT INTO sys_user_role VALUES(8101,8102),(8101,8103),(8101,8104);
 UPDATE sys_menu SET menu_type='M',perms='',order_num=2 WHERE id=8101;
 UPDATE sys_menu SET parent_id=8101,perms='p1:shared',order_num=3 WHERE id=8102;
 UPDATE sys_menu SET perms='p1:shared',order_num=1 WHERE id=8103;
 INSERT INTO sys_menu(id,menu_name,menu_type,visible,status,perms,parent_id) VALUES
 (8104,'button','F',0,1,'p1:button',0),(8105,'hidden','C',0,1,'p1:hidden',0),
 (8106,'disabled','C',1,2,'p1:disabled',0),(8107,'invalid roles','C',1,1,'p1:invalid',0),
 (8108,'orphan','C',1,1,'p1:orphan',8105);
 INSERT INTO sys_role_menu VALUES(8101,8102),(8102,8102),(8101,8103),(8102,8103),
 (8101,8104),(8101,8105),(8101,8106),(8101,8108),(8103,8107),(8104,8107);`)
	keys, err := r.RoleKeysOf(ctx, 8101)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"p1-one", "p1-two"}) {
		t.Fatalf("keys=%v", keys)
	}
	perms, err := r.PermsOf(ctx, 8101)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(perms)
	if !reflect.DeepEqual(perms, []string{"p1:button", "p1:hidden", "p1:orphan", "p1:shared"}) {
		t.Fatalf("perms=%v", perms)
	}
	menus, err := r.MenusOf(ctx, 8101)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, m := range menus {
		ids = append(ids, m.ID)
	}
	if !reflect.DeepEqual(ids, []int64{8103, 8101, 8102}) {
		t.Fatalf("menu ids=%v", ids)
	}
	execFixtureSQL(t, r.db, `INSERT INTO sys_user_role VALUES(8101,1)`)
	perms, err = r.PermsOf(ctx, 8101)
	if err != nil || !reflect.DeepEqual(perms, []string{"*:*:*"}) {
		t.Fatalf("admin=%v %v", perms, err)
	}
	menus, err = r.MenusOf(ctx, 8101)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range menus {
		if m.MenuType == "F" || m.Visible != 1 || m.Status != 1 || m.ID == 8108 {
			t.Fatalf("admin invalid menu=%+v", m)
		}
	}
}

func TestAccountAtomicUpdates(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	execFixtureSQL(t, r.db, `UPDATE sys_user SET pwd_reset_required=1,token_version=7 WHERE id=8101`)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- r.IncrementTokenVersion(ctx, 8101) }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if v, err := r.TokenVersion(ctx, 8101); err != nil || v != 9 {
		t.Fatalf("version=%d %v", v, err)
	}
	if err := r.SetPwdResetRequired(ctx, 8101, false); err != nil {
		t.Fatal(err)
	}
	if v, err := r.PwdResetRequired(ctx, 8101); err != nil || v {
		t.Fatalf("flag=%v %v", v, err)
	}
	if err := r.InTransaction(ctx, func(tx *Repo) error {
		var stamp time.Time
		if database.DriverOf(tx.db) == "sqlite" {
			stamp = time.Now().UTC().Truncate(time.Second)
		} else if err := tx.db.Raw("SELECT CURRENT_TIMESTAMP").Scan(&stamp).Error; err != nil {
			return err
		}
		if err := tx.IncrementTokenVersion(ctx, 8101); err != nil {
			return err
		}
		if err := tx.SetPwdResetRequired(ctx, 8101, true); err != nil {
			return err
		}
		if err := tx.ChangeOwnPassword(ctx, 8101, "changed-hash"); err != nil {
			return err
		}
		var u model.SysUser
		if err := tx.db.First(&u, 8101).Error; err != nil {
			return err
		}
		if (database.DriverOf(tx.db) == "postgres" && !u.UpdateTime.Equal(stamp)) || u.UpdateTime.Before(stamp) || u.PwdResetRequired != 0 || u.TokenVersion != 11 || u.Password != "changed-hash" {
			t.Fatalf("account=%+v stamp=%v", u, stamp)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{-1, 8102} {
		if id == 8102 {
			execFixtureSQL(t, r.db, "UPDATE sys_user SET del_flag=1 WHERE id=8102")
		}
		for _, call := range []func() error{
			func() error { return r.IncrementTokenVersion(ctx, id) }, func() error { return r.SetPwdResetRequired(ctx, id, false) },
			func() error { return r.ChangeOwnPassword(ctx, id, "hash") }, func() error { return r.ChangeOwnPasswordAndRevokeRefresh(ctx, id, "hash", nil) },
		} {
			if err := call(); !errors.Is(err, ErrNotFound) {
				t.Fatalf("id %d: %v", id, err)
			}
		}
	}
}

func TestPasswordAndRefreshAtomicity(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "revoke_failure"
		}
		t.Run(name, func(t *testing.T) {
			r := newRepositoryFixture(t)
			ctx := context.Background()
			execFixtureSQL(t, r.db, `UPDATE sys_user SET pwd_reset_required=1,token_version=7 WHERE id=8101`)
			execFixtureSQL(t, r.db, `INSERT INTO sys_refresh_token(token_hash,user_id,expires_at) VALUES('p1-token',8101,?)`, time.Now().UTC().Add(time.Hour))
			if fail {
				testutil.AddConstraint(t, r.db, "sys_refresh_token", "p1_reject_revoke", "CHECK(revoked=0)")
			}
			err := r.ChangeOwnPasswordAndRevokeRefresh(ctx, 8101, "new-hash", NewRefreshTokenRepo(r.db))
			if fail && err == nil {
				t.Fatal("revoke failure missing")
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			var u model.SysUser
			if err := r.db.First(&u, 8101).Error; err != nil {
				t.Fatal(err)
			}
			wantHash := "new-hash"
			wantVersion := int32(8)
			wantFlag := int16(0)
			wantRevoked := int64(1)
			if fail {
				wantHash = "old-hash"
				wantVersion = 7
				wantFlag = 1
				wantRevoked = 0
			}
			if u.Password != wantHash || u.TokenVersion != wantVersion || u.PwdResetRequired != wantFlag {
				t.Fatalf("account=%+v", u)
			}
			if n := countFixtureRows(t, r.db, "sys_refresh_token", "token_hash='p1-token' AND revoked=?", wantRevoked); n != 1 {
				t.Fatalf("token=%d", n)
			}
			if err := r.ChangeOwnPasswordAndRevokeRefresh(ctx, 8101, "optional-hash", nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResolveScopeGrouping(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	execFixtureSQL(t, r.db, `UPDATE sys_user SET dept_id=1 WHERE id=8101;
 UPDATE sys_role SET data_scope=2 WHERE id IN (8101,8102);
 UPDATE sys_role SET data_scope=5 WHERE id=8103;
 DELETE FROM sys_role_dept WHERE role_id IN(8101,8102);
 INSERT INTO sys_role_dept VALUES(8101,2),(8101,3);
 INSERT INTO sys_role(id,role_name,role_key,data_scope,status,del_flag) VALUES
 (8104,'disabled','p1-disabled',1,2,0),(8105,'deleted','p1-deleted',1,1,1),
 (8106,'dept','p1-dept',3,1,0),(8107,'below','p1-below',4,1,0),(8108,'all','p1-all',1,1,0);
 INSERT INTO sys_dept(id,dept_name,ancestors) VALUES(8201,'child','0,1'),(8202,'grandchild','0,1,8201');`)
	var output bytes.Buffer
	logged := r.WithDB(r.db.Session(&gorm.Session{Logger: logger.New(log.New(&output, "", 0), logger.Config{LogLevel: logger.Info})}))
	for _, tc := range []struct {
		name      string
		ids       []int64
		all, self bool
		depts     []int64
	}{
		{name: "no_roles", self: true},
		{name: "custom", ids: []int64{8101}, depts: []int64{2, 3}},
		{name: "empty_custom", ids: []int64{8102}},
		{name: "self", ids: []int64{8103}, self: true},
		{name: "inactive", ids: []int64{8104, 8105}, self: true},
		{name: "merged", ids: []int64{8101, 8102, 8103, 8104, 8105}, self: true, depts: []int64{2, 3}},
		{name: "dept", ids: []int64{8106}, depts: []int64{1}},
		{name: "below", ids: []int64{8107}, depts: []int64{1, 8201, 8202}},
		{name: "all", ids: []int64{8101, 8108}, all: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.InTransaction(ctx, func(tx *Repo) error { return replaceUserRoles(tx.db, 8101, tc.ids) }); err != nil {
				t.Fatal(err)
			}
			output.Reset()
			got, err := logged.ResolveScope(ctx, 8101)
			if err != nil {
				t.Fatal(err)
			}
			if got.All != tc.all || got.SelfOnly != tc.self || got.UserID != 8101 || (!tc.all && !intSliceEqualSorted(got.DeptIDs, tc.depts)) {
				t.Fatalf("scope=%+v", got)
			}
			count := strings.Count(strings.ReplaceAll(output.String(), "`", `"`), `FROM "sys_role"`)
			if count != 1 {
				t.Fatalf("role queries=%d SQL=%s", count, output.String())
			}
			t.Logf("roles=%d relation_queries=%d scope=%+v", len(tc.ids), count, got)
		})
	}
	execFixtureSQL(t, r.db, `ALTER TABLE sys_role_dept RENAME COLUMN dept_id TO fixture_dept_id`)
	if _, err := r.ResolveScope(ctx, 8101); err == nil {
		t.Fatal("query error missing")
	}
}

func TestCleanLogsAndReset(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	execFixtureSQL(t, r.db, `INSERT INTO sys_oper_log(title) VALUES('p1'); INSERT INTO sys_login_log(username) VALUES('p1');
 INSERT INTO sys_login_attempt(username,fail_count) VALUES('alice',1),('other',2),('',3)`)
	if err := r.CleanOperLogs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_oper_log", "1=1"); n != 0 {
		t.Fatalf("oper logs=%d", n)
	}
	if n := countFixtureRows(t, r.db, "sys_login_log", "username='p1'"); n != 1 {
		t.Fatalf("login logs=%d", n)
	}
	if err := r.CleanLoginLogs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_login_log", "1=1"); n != 0 {
		t.Fatalf("login logs=%d", n)
	}
	if n := countFixtureRows(t, r.db, "sys_user", "id=8101 AND del_flag=0"); n != 1 {
		t.Fatalf("user=%d", n)
	}
	for _, m := range []any{&model.SysOperLog{}, &model.SysLoginLog{}} {
		if err := r.db.Model(m).Update("status", 2).Error; !errors.Is(err, gorm.ErrMissingWhereClause) {
			t.Fatalf("global protection=%v", err)
		}
	}
	security := NewLoginSecurity(r.db, 3, 900)
	if err := security.Reset(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_login_attempt", "username='' AND fail_count=3"); n != 1 {
		t.Fatal("empty reset changed row")
	}
	if err := security.Reset(ctx, "Alice"); err != nil {
		t.Fatal(err)
	}
	if n := countFixtureRows(t, r.db, "sys_login_attempt", "username='alice'"); n != 0 {
		t.Fatal("account retained")
	}
	if n := countFixtureRows(t, r.db, "sys_login_attempt", "username='other' AND fail_count=2"); n != 1 {
		t.Fatal("unrelated counter changed")
	}
	if err := security.Reset(ctx, "missing"); err != nil {
		t.Fatal(err)
	}
}
