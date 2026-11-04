//go:build integration

package product_test

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"connectrpc.com/connect/v2"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
	gen "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/resource"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestRepositoryListAndUpdateErrors(t *testing.T) {
	db := producttest.OpenMigrated(t)
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	repo := gen.NewProductRepository()
	ctx := context.Background()
	all := resource.UnrestrictedAccessScope()
	m := &model.ProductItem{ProductNo: "G-1", Name: "Alpha%", PriceCents: 12, DeptID: 1, CreateBy: 1, Remark: "keep"}
	if err := db.Create(m).Error; err != nil {
		t.Fatal(err)
	}
	zero, one := int64(0), int64(1)
	for _, q := range []gen.ProductListQuery{{}, {Keyword: "%"}, {Keyword: "ALPHA", DeptId: &one}, {DeptId: &zero}, {PageNum: 3, PageSize: 1}, {Orders: []resource.Order{{Column: gen.ProductColumnPriceCents, Desc: true}}}} {
		for scopeIndex, scope := range []resource.AccessScope{all, {}, resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("dept_id = ?", 2) })} {
			got, total, err := repo.List(ctx, store, q, scope)
			wantTotal := int64(0)
			if scopeIndex == 0 && (q.DeptId == nil || *q.DeptId == 1) {
				wantTotal = 1
			}
			wantRows := int(wantTotal)
			if q.PageNum == 3 {
				wantRows = 0
			}
			if err != nil || total != wantTotal || len(got) != wantRows {
				t.Fatalf("list q=%+v: %v %d %v; want rows=%d total=%d", q, got, total, err, wantRows, wantTotal)
			}
			if len(got) == 1 && got[0].ID != m.ID {
				t.Fatalf("unexpected row: %+v", got[0])
			}
		}
	}
	for _, scope := range []resource.AccessScope{{}, resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("dept_id = ?", 2) })} {
		changed := *m
		changed.Remark = "denied"
		err := store.InTransaction(ctx, func(tx resource.Tx) error {
			return repo.Update(ctx, tx, m.ID, &changed, resource.NewColumnSet(gen.ProductColumnRemark), scope)
		})
		if !errors.Is(err, resource.ErrNotFound) {
			t.Fatalf("scope update: %v", err)
		}
	}
	// A scope change between lock and write must also constrain the UPDATE.
	calls := 0
	writeDenied := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB {
		calls++
		if calls == 1 {
			return db
		}
		return db.Where("dept_id = ?", 2)
	})
	changed := *m
	changed.Remark = "write-denied"
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		return repo.Update(ctx, tx, m.ID, &changed, resource.NewColumnSet(gen.ProductColumnRemark), writeDenied)
	})
	if !errors.Is(err, resource.ErrNotFound) || calls != 3 {
		t.Fatalf("write scope: calls=%d err=%v", calls, err)
	}
	if err := repo.Update(ctx, store, m.ID, m, resource.NewColumnSet(), all); err == nil || err.Error() != gen.ProductValidateUpdateColumns(resource.NewColumnSet()).Error() {
		t.Fatalf("empty patch: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := repo.Update(cancelled, store, m.ID, m, resource.NewColumnSet(gen.ProductColumnRemark), all); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	missing := *m
	if err := repo.Update(ctx, store, -1, &missing, resource.NewColumnSet(gen.ProductColumnRemark), all); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	rollback := errors.New("rollback update")
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		changed := *m
		changed.Remark = "uncommitted"
		if err := repo.Update(ctx, tx, m.ID, &changed, resource.NewColumnSet(gen.ProductColumnRemark), all); err != nil {
			return err
		}
		inside, err := repo.GetByKey(ctx, tx, m.ID, all)
		if err != nil {
			return err
		}
		if inside.Remark != "uncommitted" {
			t.Fatal("transaction read")
		}
		outside, err := repo.GetByKey(ctx, store, m.ID, all)
		if err != nil {
			return err
		}
		if outside.Remark != "keep" {
			t.Fatal("outside visibility")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	// Selected zero values persist; omitted fields and rollback retain their values.
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		changed := *m
		changed.Remark = ""
		changed.PriceCents = 0
		cols := resource.NewColumnSet(gen.ProductColumnRemark, gen.ProductColumnPriceCents)
		if err := repo.Update(ctx, tx, m.ID, &changed, cols, all); err != nil {
			return err
		}
		saved, err := repo.GetByKey(ctx, tx, m.ID, all)
		if err != nil {
			return err
		}
		if saved.Name != m.Name || saved.PriceCents != 0 || saved.Remark != "" ||
			!saved.UpdateTime.Equal(changed.UpdateTime) || !saved.UpdateTime.After(m.UpdateTime) {
			t.Fatalf("selective update: %+v", saved)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	// Force an error at the write statement, after the locking read succeeds.
	producttest.AddConstraint(t, db, "product_item", "remark_validation", "CHECK (remark <> 'reject')")
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		changed := *m
		changed.Remark = "reject"
		return repo.Update(ctx, tx, m.ID, &changed, resource.NewColumnSet(gen.ProductColumnRemark), all)
	})
	code := uint16(3819)
	if database.DriverOf(db) == "mariadb" {
		code = 4025
	}
	producttest.RequireNativeError(t, db, err, "23514", code)
	saved, err := repo.GetByKey(ctx, store, m.ID, all)
	if err != nil || saved.Remark != "keep" {
		t.Fatalf("rollback: %+v %v", saved, err)
	}
}

func TestUpdateRPCAuthorization(t *testing.T) {
	db := producttest.OpenMigrated(t)
	env := startApp(t, db, product.ModuleOptions{})
	admin := env.login(t, "admin", "admin123")
	created, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{ProductNo: "G-AUTH", Name: "keep", DeptId: 1})
	if err != nil {
		t.Fatal(err)
	}
	id := created.GetItem().GetId()
	remark := "denied"
	_, err = env.products.UpdateProduct(context.Background(), &productv1.UpdateProductRequest{Id: id, Remark: &remark})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous: %v", err)
	}
	dept := createDept(t, env, admin, "G-other")
	editor := createScopedUser(t, env, db, "g-editor", dept, "biz_product_editor")
	_, err = env.products.UpdateProduct(authed(t, editor), &productv1.UpdateProductRequest{Id: id, Remark: &remark})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("cross-dept: %v", err)
	}
	reader := createScopedUser(t, env, db, "g-reader", 1, "biz_product_reader")
	_, err = env.products.UpdateProduct(authed(t, reader), &productv1.UpdateProductRequest{Id: id, Remark: &remark})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("reader: %v", err)
	}
	got, err := env.products.GetProduct(authed(t, admin), &productv1.GetProductRequest{Id: id})
	if err != nil || got.GetItem().GetRemark() != "" {
		t.Fatalf("denied write persisted: %v", err)
	}
}

func TestUpdateLifecycleAndCommitFailure(t *testing.T) {
	db := producttest.OpenMigrated(t)
	stats := &product.CommitStats{}
	var failAfterInsert atomic.Bool
	var failCommit atomic.Bool
	var observed atomic.Int64
	writer := product.EventWriter(func(ctx context.Context, tx resource.Tx, id, actor int64, action string) error {
		if action == "update" {
			var inside, outside model.ProductItem
			if err := tx.DB().First(&inside, id).Error; err != nil {
				return err
			}
			if err := db.First(&outside, id).Error; err != nil {
				return err
			}
			if inside.Name == outside.Name || !inside.UpdateTime.After(outside.UpdateTime) {
				return errors.New("hook did not observe uncommitted update")
			}
			observed.Add(1)
		}
		if err := tx.DB().WithContext(ctx).Exec(`INSERT INTO product_event(product_id,actor_id,action) VALUES (?,?,?)`, id, actor, action).Error; err != nil {
			return err
		}
		if failCommit.Load() {
			return tx.DB().Rollback().Error
		}
		if failAfterInsert.Load() {
			return errors.New("after update event failure")
		}
		return nil
	})
	env := startApp(t, db, product.ModuleOptions{EventWriter: writer, CommitStats: stats})
	admin := env.login(t, "admin", "admin123")
	created, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{ProductNo: "G-HOOK", Name: "old", PriceCents: 7, Remark: "keep", DeptId: 1})
	if err != nil {
		t.Fatal(err)
	}
	id := created.GetItem().GetId()
	var original model.ProductItem
	if err := db.First(&original, id).Error; err != nil {
		t.Fatal(err)
	}
	// Omitted optional fields yield an empty patch and preserve audit time.
	_, err = env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{Id: id})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty update: %v", err)
	}
	var saved model.ProductItem
	if err := db.First(&saved, id).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.UpdateTime.Equal(original.UpdateTime) || observed.Load() != 0 || stats.Updates.Load() != 0 {
		t.Fatal("empty patch lifecycle")
	}
	blank := "   "
	_, err = env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{Id: id, Name: &blank})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || observed.Load() != 0 {
		t.Fatalf("BeforeUpdate failure: %v", err)
	}
	name, empty, zero := "  normalized  ", "", int64(0)
	updated, err := env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{Id: id, Name: &name, Remark: &empty, PriceCents: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.First(&saved, id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Name != "normalized" || saved.Remark != "" || saved.PriceCents != 0 || !saved.UpdateTime.After(original.UpdateTime) || !saved.UpdateTime.Equal(updated.GetItem().GetUpdateTime().AsTime()) || stats.Updates.Load() != 1 || observed.Load() != 1 {
		t.Fatalf("update snapshot: %+v", saved)
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action='update'`) != 1 {
		t.Fatal("missing update event")
	}
	committed := saved
	failAfterInsert.Store(true)
	name = "after-failure"
	_, err = env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{Id: id, Name: &name})
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("AfterUpdate failure: %v", err)
	}
	if err := db.First(&saved, id).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, committed) || stats.Updates.Load() != 1 || producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action='update'`) != 1 {
		t.Fatal("AfterUpdate rollback")
	}
	failAfterInsert.Store(false)
	// The event insert succeeds; the deferred unique constraint fails at commit.
	if database.DriverOf(db) != "postgres" {
		failCommit.Store(true)
	} else {
		producttest.MustExec(t, db, `ALTER TABLE product_event ADD CONSTRAINT event_uniqueness UNIQUE(product_id,action) DEFERRABLE INITIALLY DEFERRED`)
	}
	name = "commit-failure"
	_, err = env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{Id: id, Name: &name})
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("commit failure: %v", err)
	}
	if err := db.First(&saved, id).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, committed) || stats.Updates.Load() != 1 || observed.Load() != 3 || producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action='update'`) != 1 {
		t.Fatal("commit rollback / AfterCommit")
	}
}
