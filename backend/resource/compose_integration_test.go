//go:build integration

package resource

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestComposedHooksShareTransactionAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	if err := db.AutoMigrate(&composedModel{}); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("business after failure")
	var handle *gorm.DB
	committed := 0
	systemAfter := 0
	h := ComposeHooks(&HookSet[composedModel]{
		BeforeCreate: func(ctx context.Context, tx Tx, m *composedModel) error {
			handle = tx.DB()
			return tx.DB().WithContext(ctx).Create(&composedModel{Name: "system-event"}).Error
		},
		AfterCreate:       func(context.Context, Tx, *composedModel) error { systemAfter++; return nil },
		AfterCommitCreate: func(context.Context, composedModel) { committed++ },
	}, &HookSet[composedModel]{AfterCreate: func(_ context.Context, tx Tx, _ *composedModel) error {
		if tx.DB() != handle {
			t.Fatal("transaction changed")
		}
		return sentinel
	}})
	ctx := context.Background()
	m := &composedModel{Name: "business"}
	err = store.InTransaction(ctx, func(tx Tx) error {
		if err := h.BeforeCreate(ctx, tx, m); err != nil {
			return err
		}
		if err := tx.DB().WithContext(ctx).Create(m).Error; err != nil {
			return err
		}
		return h.AfterCreate(ctx, tx, m)
	})
	if err == nil {
		h.AfterCommitCreate(ctx, *m)
	}
	var count int64
	if e := db.Model(&composedModel{}).Count(&count).Error; e != nil {
		t.Fatal(e)
	}
	if !errors.Is(err, sentinel) || count != 0 || committed != 0 || systemAfter != 0 {
		t.Fatalf("err=%v rows=%d committed=%d after=%d", err, count, committed, systemAfter)
	}
}
