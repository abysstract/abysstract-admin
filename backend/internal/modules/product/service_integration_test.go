//go:build integration

package product_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/system"
)

func TestProductServiceCRUDScopeAndEvents(t *testing.T) {
	db := producttest.OpenMigrated(t)
	stats := &product.CommitStats{}
	env := startApp(t, db, product.ModuleOptions{CommitStats: stats})
	admin := env.login(t, "admin", "admin123")

	_, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo:  "  ab-100 ",
		Name:       "  首份商品  ",
		PriceCents: 0,
		DeptId:     1,
		Remark:     "keep-me",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if stats.Creates.Load() != 1 {
		t.Fatalf("after commit creates = %d", stats.Creates.Load())
	}
	no := producttest.QueryString(t, db, `SELECT product_no FROM product_item WHERE product_no = 'ab-100'`)
	if no != "ab-100" {
		t.Fatalf("normalized no = %q", no)
	}
	createdBy := producttest.QueryInt64(t, db, `SELECT create_by FROM product_item WHERE product_no = 'ab-100'`)
	if createdBy != 1 {
		t.Fatalf("create_by = %d", createdBy)
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action = 'create'`) != 1 {
		t.Fatal("missing create event")
	}

	_, err = env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-100",
		Name:      "冲突",
		DeptId:    1,
	})
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("case conflict = %v", err)
	}

	_, err = env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-BLANK",
		Name:      "   ",
		DeptId:    1,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("blank name = %v", err)
	}

	_, err = env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo:  "AB-NEG",
		Name:       "负",
		PriceCents: -1,
		DeptId:     1,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("negative amount = %v", err)
	}

	_, err = env.products.CreateProduct(context.Background(), &productv1.CreateProductRequest{ProductNo: "AB-ANON", Name: "未登录", DeptId: 1})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous create = %v", err)
	}

	large, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo:  "AB-BIG",
		Name:       "大整数",
		PriceCents: 9007199254740993,
		DeptId:     1,
	})
	if err != nil {
		t.Fatalf("large create: %v", err)
	}
	if large.GetItem().GetPriceCents() != 9007199254740993 {
		t.Fatalf("amount = %d", large.GetItem().GetPriceCents())
	}

	id := large.GetItem().GetId()
	emptyRemark := ""
	updated, err := env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{
		Id:     id,
		Remark: &emptyRemark,
	})
	if err != nil {
		t.Fatalf("clear remark: %v", err)
	}
	if updated.GetItem().GetName() != "大整数" {
		t.Fatalf("omitted name changed: %q", updated.GetItem().GetName())
	}
	if updated.GetItem().GetRemark() != "" {
		t.Fatalf("remark = %q", updated.GetItem().GetRemark())
	}

	zero := int64(0)
	updated, err = env.products.UpdateProduct(authed(t, admin), &productv1.UpdateProductRequest{
		Id:         id,
		PriceCents: &zero,
	})
	if err != nil {
		t.Fatalf("zero amount update: %v", err)
	}
	if updated.GetItem().GetPriceCents() != 0 {
		t.Fatalf("zero amount not stored")
	}

	dept2 := createDept(t, env, admin, "分部")
	editorToken := createScopedUser(t, env, db, "editor1", dept2, "biz_product_editor")
	_, err = env.products.CreateProduct(authed(t, editorToken), &productv1.CreateProductRequest{
		ProductNo: "ED-CROSS",
		Name:      "跨部门",
		DeptId:    1,
	})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-dept create = %v", err)
	}
	created, err := env.products.CreateProduct(authed(t, editorToken), &productv1.CreateProductRequest{
		ProductNo: "ED-OWN",
		Name:      "本部门",
		DeptId:    dept2,
	})
	if err != nil {
		t.Fatalf("same-dept create: %v", err)
	}

	listed, err := env.products.ListProducts(authed(t, editorToken), &productv1.ListProductsRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 20},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, item := range listed.GetItems() {
		if item.GetDeptId() != dept2 {
			t.Fatalf("list leaked dept %d", item.GetDeptId())
		}
	}
	if _, err := env.products.GetProduct(authed(t, editorToken), &productv1.GetProductRequest{Id: id}); connect.CodeOf(err) != connect.CodeNotFound && connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-dept get = %v", err)
	}

	readerToken := createScopedUser(t, env, db, "reader1", 1, "biz_product_reader")
	_, err = env.products.CreateProduct(authed(t, readerToken), &productv1.CreateProductRequest{
		ProductNo: "RD-WRITE",
		Name:      "只读写",
		DeptId:    1,
	})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("reader write = %v", err)
	}

	selfToken := createSelfAndDeptUser(t, env, db, "union1", 1, dept2)
	_, err = env.products.CreateProduct(authed(t, selfToken), &productv1.CreateProductRequest{
		ProductNo: "UN-SELF",
		Name:      "并集本人",
		DeptId:    dept2,
	})
	if err != nil {
		t.Fatalf("union create: %v", err)
	}

	_, err = env.products.DeleteProduct(authed(t, admin), &productv1.DeleteProductRequest{Id: created.GetItem().GetId()})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action = 'delete'`) < 1 {
		t.Fatal("delete event missing")
	}

	second, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-BATCH-1",
		Name:      "批1",
		DeptId:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	third, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-BATCH-2",
		Name:      "批2",
		DeptId:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.products.BatchDeleteProducts(authed(t, admin), &productv1.BatchDeleteProductsRequest{
		Ids: []int64{second.GetItem().GetId(), third.GetItem().GetId(), 999999},
	})
	if err == nil {
		t.Fatal("batch delete with missing id succeeded")
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_item WHERE product_no IN ('ab-batch-1', 'ab-batch-2')`) != 2 {
		t.Fatal("partial batch delete")
	}

	_, err = env.products.BatchDeleteProducts(authed(t, admin), &productv1.BatchDeleteProductsRequest{
		Ids: []int64{second.GetItem().GetId(), third.GetItem().GetId()},
	})
	if err != nil {
		t.Fatalf("batch delete: %v", err)
	}

	actor, ok := identity.FromContext(context.Background())
	if ok {
		t.Fatalf("background identity leaked: %+v", actor)
	}
}

func TestBatchDeleteNthEventFailureRollsBackProductsAndEvents(t *testing.T) {
	db := producttest.OpenMigrated(t)
	stats := &product.CommitStats{}
	var deleteWrites atomic.Int64
	writer := product.EventWriter(func(ctx context.Context, tx resource.Tx, productID, actorID int64, action string) error {
		if action == "delete" {
			n := deleteWrites.Add(1)
			if n >= 2 {
				return errors.New("forced batch event failure")
			}
		}
		return tx.DB().WithContext(ctx).Exec(
			`INSERT INTO product_event (product_id, actor_id, action) VALUES (?, ?, ?)`,
			productID, actorID, action,
		).Error
	})
	env := startApp(t, db, product.ModuleOptions{EventWriter: writer, CommitStats: stats})
	admin := env.login(t, "admin", "admin123")
	first, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-BT-1",
		Name:      "批删事件1",
		DeptId:    1,
	})
	if err != nil {
		t.Fatalf("create 1: %v", err)
	}
	second, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-BT-2",
		Name:      "批删事件2",
		DeptId:    1,
	})
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	_, err = env.products.BatchDeleteProducts(authed(t, admin), &productv1.BatchDeleteProductsRequest{
		Ids: []int64{first.GetItem().GetId(), second.GetItem().GetId()},
	})
	if err == nil {
		t.Fatal("batch delete succeeded after nth event failure")
	}
	if deleteWrites.Load() < 2 {
		t.Fatalf("delete event attempts = %d, want at least 2", deleteWrites.Load())
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_item WHERE product_no IN ('ab-bt-1', 'ab-bt-2')`) != 2 {
		t.Fatal("batch delete left a partial product result")
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_event WHERE action = 'delete'`) != 0 {
		t.Fatal("delete events remained after rollback")
	}
	if stats.Batches.Load() != 0 {
		t.Fatalf("AfterCommitBatchDelete ran: %d", stats.Batches.Load())
	}
}

func TestEventFailureRollsBackAndSkipsAfterCommit(t *testing.T) {
	db := producttest.OpenMigrated(t)
	stats := &product.CommitStats{}
	failing := product.EventWriter(func(context.Context, resource.Tx, int64, int64, string) error {
		return errors.New("forced event failure")
	})
	env := startApp(t, db, product.ModuleOptions{EventWriter: failing, CommitStats: stats})
	admin := env.login(t, "admin", "admin123")
	_, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "AB-FAIL",
		Name:      "失败",
		DeptId:    1,
	})
	if err == nil {
		t.Fatal("create succeeded with failing events")
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_item WHERE product_no = 'ab-fail'`) != 0 {
		t.Fatal("product row survived event failure")
	}
	if producttest.Count(t, db, `SELECT count(*) FROM product_event`) != 0 {
		t.Fatal("event row survived event failure")
	}
	if stats.Creates.Load() != 0 {
		t.Fatalf("after-commit ran: %d", stats.Creates.Load())
	}
}

func TestAppShutdownClosesCleanly(t *testing.T) {
	db := producttest.OpenMigrated(t)
	systemDef, err := system.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	productDef, err := product.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	app, err := adminapp.New(adminapp.Options{
		Config:  producttest.TestConfig(t, producttest.CurrentDSN(t)),
		DB:      db,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Modules: []module.Definition{systemDef, productDef},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timeout")
	}
	if _, err := db.DB(); err != nil {
		t.Fatalf("borrowed db: %v", err)
	}
}

func TestProductEventConstraintFailureRollsBackHook(t *testing.T) {
	db := producttest.OpenMigrated(t)
	producttest.AddConstraint(t, db, "product_event", "p1_reject_create", "CHECK(action <> 'create')")
	stats := &product.CommitStats{}
	env := startApp(t, db, product.ModuleOptions{CommitStats: stats})
	admin := env.login(t, "admin", "admin123")
	_, err := env.products.CreateProduct(authed(t, admin), &productv1.CreateProductRequest{
		ProductNo: "P1-FAIL", Name: "P1", DeptId: 1,
	})
	if err == nil {
		t.Fatal("create succeeded despite event constraint")
	}
	if n := producttest.Count(t, db, `SELECT count(*) FROM product_item WHERE product_no='p1-fail'`); n != 0 {
		t.Fatalf("products=%d", n)
	}
	if n := producttest.Count(t, db, `SELECT count(*) FROM product_event`); n != 0 {
		t.Fatalf("events=%d", n)
	}
	if stats.Creates.Load() != 0 {
		t.Fatalf("AfterCommit=%d", stats.Creates.Load())
	}
}

func TestMissingPermissionFailsStartup(t *testing.T) {
	systemDef, err := system.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	productDef, err := product.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	productDef.Permissions = func(reg permission.Registrar) error {
		return reg.RegisterGenerated(permission.Entry{
			Procedure:  "/company.product.v1.ProductService/ListProducts",
			Permission: "biz:product:list",
		})
	}
	_, err = adminapp.New(adminapp.Options{
		Config:  producttest.TestConfig(t, "host=localhost user=test password=test dbname=test sslmode=disable"),
		Modules: []module.Definition{systemDef, productDef},
	})
	if err == nil || !strings.Contains(err.Error(), "missing declaration") {
		t.Fatalf("err = %v", err)
	}
}
