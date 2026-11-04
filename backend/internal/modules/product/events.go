package product

import (
	"context"
	"fmt"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

const (
	eventCreate = "create"
	eventUpdate = "update"
	eventDelete = "delete"
)

// EventWriter writes one product event inside the current resource transaction.
type EventWriter func(ctx context.Context, tx resource.Tx, productID, actorID int64, action string) error

func writeProductEvent(ctx context.Context, tx resource.Tx, productID, actorID int64, action string) error {
	if tx.DB() == nil {
		return fmt.Errorf("product event: transaction is required")
	}
	if productID == 0 {
		return fmt.Errorf("product event: product id is required")
	}
	if actorID == 0 {
		return fmt.Errorf("product event: actor id is required")
	}
	// Keep the event timestamp supplied by the database default.
	return tx.DB().WithContext(ctx).Omit("created_at").Create(&model.ProductEvent{
		ProductID: productID, ActorID: actorID, Action: action,
	}).Error
}

func actorID(ctx context.Context) (int64, error) {
	actor, ok := identity.FromContext(ctx)
	if !ok || actor.UserID == 0 {
		return 0, errUnauthenticated
	}
	return actor.UserID, nil
}

func writeEventFor(writer EventWriter, action string) func(context.Context, resource.Tx, *model.ProductItem) error {
	return func(ctx context.Context, tx resource.Tx, m *model.ProductItem) error {
		if m == nil {
			return fmt.Errorf("product event: model is required")
		}
		actor, err := actorID(ctx)
		if err != nil {
			return err
		}
		return writer(ctx, tx, m.ID, actor, action)
	}
}

func writeEventsFor(writer EventWriter, action string) func(context.Context, resource.Tx, []*model.ProductItem) error {
	return func(ctx context.Context, tx resource.Tx, models []*model.ProductItem) error {
		actor, err := actorID(ctx)
		if err != nil {
			return err
		}
		for _, m := range models {
			if m == nil {
				return fmt.Errorf("product event: model is required")
			}
			if err := writer(ctx, tx, m.ID, actor, action); err != nil {
				return err
			}
		}
		return nil
	}
}
