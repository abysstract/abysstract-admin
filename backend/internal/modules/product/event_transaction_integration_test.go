//go:build integration

package product

import (
	"context"
	"errors"
	"testing"
	"time"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestProductEventCallerTransaction(t *testing.T) {
	db := producttest.OpenMigrated(t)
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("caller rollback after event")
	ctx := context.Background()
	err = store.InTransaction(ctx, func(tx resource.Tx) error {
		m := &model.ProductItem{ProductNo: "P1-ROLLBACK", Name: "P1", DeptID: 1, CreateBy: 1}
		if err := tx.DB().Create(m).Error; err != nil {
			return err
		}
		var stamp time.Time
		if database.DriverOf(db) == "sqlite" {
			stamp = time.Now().UTC().Truncate(time.Second)
		} else if err := tx.DB().Raw("SELECT CURRENT_TIMESTAMP").Scan(&stamp).Error; err != nil {
			return err
		}
		if err := writeProductEvent(ctx, tx, m.ID, 1, eventCreate); err != nil {
			return err
		}
		var event model.ProductEvent
		if err := tx.DB().Where("product_id = ?", m.ID).Take(&event).Error; err != nil {
			return err
		}
		var end time.Time
		query := "SELECT CURRENT_TIMESTAMP"
		if database.DriverOf(db) == "mysql" || database.DriverOf(db) == "mariadb" {
			query = "SELECT CURRENT_TIMESTAMP(6)"
		}
		if database.DriverOf(db) == "sqlite" {
			end = time.Now().UTC()
		} else if err := tx.DB().Raw(query).Scan(&end).Error; err != nil {
			return err
		}
		if event.ActorID != 1 || event.Action != eventCreate || event.CreatedAt.Before(stamp) || event.CreatedAt.After(end) || (database.DriverOf(db) == "postgres" && !event.CreatedAt.Equal(stamp)) {
			t.Fatalf("event=%+v transaction_time=%v", event, stamp)
		}
		if count := producttest.Count(t, db, "SELECT count(*) FROM product_event"); count != 0 {
			t.Fatalf("uncommitted events visible outside: %d", count)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if n := producttest.Count(t, db, "SELECT count(*) FROM product_item WHERE product_no='p1-rollback'"); n != 0 {
		t.Fatalf("products=%d", n)
	}
	if n := producttest.Count(t, db, "SELECT count(*) FROM product_event"); n != 0 {
		t.Fatalf("events=%d", n)
	}
}
