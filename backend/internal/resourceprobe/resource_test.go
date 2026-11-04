//go:build integration

package resourceprobe_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/resourceprobe/model"
	"github.com/abysstract/abysstract-admin/backend/internal/resourceprobe/query"
	"github.com/abysstract/abysstract-admin/backend/internal/testdb"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestPublicResourceTransaction(t *testing.T) {
	fixture := testdb.Open(t)
	store, err := resource.NewStore(fixture.DB.Table(fixture.Product))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	scope := resource.UnrestrictedAccessScope()
	row := model.Record{Name: "public resource"}
	sentinel := errors.New("rollback")
	if err := store.InTransaction(ctx, func(tx resource.Tx) error {
		if err := resource.NormalizeDBError(query.Use(tx.DB().WithContext(ctx)).Record.Table(fixture.Product).WithContext(ctx).Clauses(clause.Returning{}).DO.Create(&row)); err != nil {
			return err
		}
		if row.ID == 0 {
			t.Fatal("RETURNING ID missing")
		}
		item := query.Use(scope.Apply(tx.DB().WithContext(ctx))).Record.Table(fixture.Product)
		loaded, err := item.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(item.ID.Eq(row.ID)).First()
		if err := resource.NormalizeDBError(err); err != nil {
			return err
		}
		if loaded.Name != row.Name {
			t.Fatal("transaction lookup mismatch")
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("rollback: %v", err)
	}
	lookup := query.Use(scope.Apply(store.DB().WithContext(ctx))).Record.Table(fixture.Product)
	if _, err := lookup.WithContext(ctx).Where(lookup.ID.Eq(row.ID)).First(); !errors.Is(resource.NormalizeDBError(err), resource.ErrNotFound) {
		t.Fatalf("rollback lookup: %v", err)
	}
	row.ID = 0
	if err := store.InTransaction(ctx, func(tx resource.Tx) error {
		return resource.NormalizeDBError(query.Use(tx.DB().WithContext(ctx)).Record.Table(fixture.Product).WithContext(ctx).Clauses(clause.Returning{}).DO.Create(&row))
	}); err != nil {
		t.Fatal(err)
	}

	item := query.Use(scope.Apply(resource.Search([]resource.Column{"name"}, "public").Apply(store.DB().WithContext(ctx)))).Record.Table(fixture.Product).As("probe")
	list := item.WithContext(ctx).Order(item.ID)
	total, err := list.Count()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := list.Limit(20).Find()
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Name != row.Name {
		t.Fatalf("list: %v %d %v", rows, total, err)
	}
	if err := store.InTransaction(ctx, func(tx resource.Tx) error {
		item := query.Use(scope.Apply(tx.DB().WithContext(ctx))).Record.Table(fixture.Product)
		result, err := item.WithContext(ctx).Where(item.ID.Eq(row.ID)).Delete()
		if err := resource.NormalizeDBError(err); err != nil {
			return err
		}
		if result.RowsAffected == 0 {
			return resource.ErrNotFound
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
