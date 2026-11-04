//go:build integration

package product

import (
	"context"
	"errors"
	"testing"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"github.com/abysstract/abysstract-admin/backend/database"

	"connectrpc.com/connect/v2"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestEnsureDeptExists(t *testing.T) {
	db := producttest.OpenMigrated(t)
	ctx := context.Background()
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	producttest.MustExec(t, db, `INSERT INTO sys_dept(id,dept_name,status,del_flag) VALUES(9007199254740993,'large',2,1)`)
	for _, c := range []struct {
		name    string
		id      int64
		missing bool
	}{
		{"existing", 1, false},
		{"absent", -1, true},
		{"zero", 0, true},
		{"bigint_inactive_deleted", 9007199254740993, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := store.InTransaction(ctx, func(tx resource.Tx) error {
				return ensureDeptExists(ctx, tx, c.id)
			})
			if c.missing {
				if connect.CodeOf(err) != connect.CodeInvalidArgument || err.Error() != "invalid_argument: 部门不存在" {
					t.Fatalf("missing department error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := store.InTransaction(ctx, func(tx resource.Tx) error {
			return ensureDeptExists(cancelled, tx, 1)
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("transaction_visibility_and_rollback", func(t *testing.T) {
		rollback := errors.New("rollback department fixture")
		err := store.InTransaction(ctx, func(tx resource.Tx) error {
			if err := tx.DB().Exec(`INSERT INTO sys_dept(id,dept_name) VALUES(80000001,'uncommitted')`).Error; err != nil {
				return err
			}
			if err := ensureDeptExists(ctx, tx, 80000001); err != nil {
				return err
			}
			if database.DriverOf(db) == "sqlite" {
				var count int64
				if err := db.Table("sys_dept").Where("id=?", 80000001).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("uncommitted row visible")
				}
			} else {
				outside := store.InTransaction(ctx, func(other resource.Tx) error {
					return ensureDeptExists(ctx, other, 80000001)
				})
				if connect.CodeOf(outside) != connect.CodeInvalidArgument {
					t.Fatalf("outside transaction: %v", outside)
				}
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		err = store.InTransaction(ctx, func(tx resource.Tx) error {
			return ensureDeptExists(ctx, tx, 80000001)
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("after rollback: %v", err)
		}
	})

	t.Run("database_error", func(t *testing.T) {
		producttest.MustExec(t, db, `ALTER TABLE sys_dept RENAME COLUMN id TO fixture_id`)
		err := store.InTransaction(ctx, func(tx resource.Tx) error {
			return ensureDeptExists(ctx, tx, 1)
		})
		producttest.RequireNativeError(t, db, err, "42703", 1054)
	})
}

func TestCreateScopeBigIntAndCallerTransaction(t *testing.T) {
	db := producttest.OpenMigrated(t)
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	const deptID = int64(9007199254740993)
	const actorID = int64(9007199254740995)
	rollback := errors.New("candidate rollback")
	err = store.InTransaction(context.Background(), func(tx resource.Tx) error {
		if err := tx.DB().Exec("INSERT INTO sys_dept(id,dept_name) VALUES(?,?)", deptID, "p3-uncommitted").Error; err != nil {
			return err
		}
		cases := []struct {
			scope resource.AccessScope
			allow bool
		}{
			{resource.NewAccessScope(func(q *gorm.DB) *gorm.DB { return q.Where("dept_id=?", deptID) }), true},
			{resource.NewAccessScope(func(q *gorm.DB) *gorm.DB { return q.Where("create_by=?", actorID) }), true},
			{resource.NewAccessScope(func(q *gorm.DB) *gorm.DB { return q.Where("create_by=?", actorID-1) }), false},
			{resource.AccessScope{}, false},
			{resource.NewAccessScope(func(q *gorm.DB) *gorm.DB {
				return q.Where("EXISTS (SELECT 1 FROM sys_dept WHERE id=candidate.dept_id AND dept_name=?)", "p3-uncommitted")
			}), true},
		}
		for i, c := range cases {
			validate := newCreateScopeValidator(candidateScopeProvider{c.scope}, "dept_id", "create_by")
			err := validate(context.Background(), tx, &model.ProductItem{DeptID: deptID, CreateBy: actorID})
			if c.allow && err != nil {
				t.Fatalf("case %d allowed candidate: %v", i, err)
			}
			if !c.allow && !errors.Is(err, resource.ErrScopeDenied) {
				t.Fatalf("case %d denied candidate: %v", i, err)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if producttest.Count(t, db, "SELECT COUNT(*) FROM sys_dept WHERE id=?", deptID) != 0 {
		t.Fatal("candidate transaction escaped rollback")
	}
}
