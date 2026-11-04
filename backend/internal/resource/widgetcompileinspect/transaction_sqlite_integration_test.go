//go:build integration

package gen

import (
	"context"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestWidgetPublicTransactionWithSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&schematest.WidgetModel{}); err != nil {
		t.Fatal(err)
	}
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	var handle *gorm.DB
	deny := true
	commit := 0
	hooks := resource.ComposeHooks(&resource.HookSet[schematest.WidgetModel]{BeforeCreate: func(_ context.Context, tx resource.Tx, m *schematest.WidgetModel) error {
		order = append(order, "system")
		handle = tx.DB()
		m.DeptID = 7
		m.UpdatedAt = time.Now()
		return nil
	}}, &resource.HookSet[schematest.WidgetModel]{BeforeCreate: func(_ context.Context, tx resource.Tx, m *schematest.WidgetModel) error {
		order = append(order, "business")
		if tx.DB() != handle {
			t.Fatal("business transaction changed")
		}
		m.CreatedBy = 1
		return nil
	}, AfterCommitCreate: func(context.Context, schematest.WidgetModel) { commit++ }})
	controller, err := NewWidgetController(WidgetControllerDeps{
		Store: store, DataScopeProvider: unrestrictedWidgetScope{}, Hooks: hooks,
		CreateScopeValidator: func(_ context.Context, tx resource.Tx, m *schematest.WidgetModel) error {
			order = append(order, "scope")
			if tx.DB() != handle || m.DeptID != 7 || m.CreatedBy != 1 {
				t.Fatal("scope validator must follow both hooks on the same Tx")
			}
			if deny {
				return resource.ErrScopeDenied
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	call := func() error {
		_, err := controller.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "offline"})
		return err
	}
	if err := call(); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("scope rejection=%v", err)
	}
	var count int64
	if err := db.Model(&schematest.WidgetModel{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 || commit != 0 || !reflect.DeepEqual(order, []string{"system", "business", "scope"}) {
		t.Fatalf("denied count=%d commit=%d order=%v", count, commit, order)
	}
	deny = false
	order = nil
	if err := call(); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&schematest.WidgetModel{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || commit != 1 || !reflect.DeepEqual(order, []string{"system", "business", "scope"}) {
		t.Fatalf("allowed count=%d commit=%d order=%v", count, commit, order)
	}
}
