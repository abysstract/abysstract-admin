// Package txprobe is a deliberately small business-side transaction example.
// It proves that a generated-style repository write and a business hook can
// share the public resource.Tx handle without importing internal system code.
package txprobe

import (
	"context"

	"github.com/abysstract/abysstract-admin/backend/internal/testdb"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

type productRow struct {
	ID   int64  `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

type eventRow struct {
	ID          int64  `gorm:"column:id"`
	ProductName string `gorm:"column:product_name"`
	EventKey    string `gorm:"column:event_key"`
}

// Create persists a product, invokes the in-transaction business hook, then
// invokes the optional after-commit callback only after InTransaction returns
// successfully. An after-commit panic is reported separately and does not
// undo committed data.
func Create(
	ctx context.Context,
	store *resource.Store,
	database *testdb.Database,
	name string,
	hook func(resource.Tx) error,
	afterCommit func(),
	reportAfterCommitPanic func(any),
) error {
	err := store.InTransaction(ctx, func(tx resource.Tx) error {
		if err := tx.DB().WithContext(ctx).Table(database.TableName("products")).Create(&productRow{Name: name}).Error; err != nil {
			return err
		}
		if hook == nil {
			return nil
		}
		return hook(tx)
	})
	if err != nil {
		return err
	}
	if afterCommit == nil {
		return nil
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil && reportAfterCommitPanic != nil {
				reportAfterCommitPanic(recovered)
			}
		}()
		afterCommit()
	}()
	return nil
}

// InsertEvent is the business callback's table write. It intentionally takes
// resource.Tx so the caller can verify the same GORM transaction is used.
func InsertEvent(ctx context.Context, tx resource.Tx, database *testdb.Database, productName, eventKey string) error {
	return tx.DB().WithContext(ctx).Table(database.TableName("events")).Create(&eventRow{
		ProductName: productName,
		EventKey:    eventKey,
	}).Error
}
