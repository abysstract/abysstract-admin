package product

import (
	"context"
	"fmt"
	"regexp"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var columnNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func newCreateScopeValidator(provider resource.DataScopeProvider, deptColumn, creatorColumn string) resource.CreateScopeValidator[model.ProductItem] {
	return func(ctx context.Context, tx resource.Tx, m *model.ProductItem) error {
		if provider == nil {
			return fmt.Errorf("product: DataScopeProvider is required")
		}
		if m == nil {
			return fmt.Errorf("product: model is required")
		}
		if tx.DB() == nil {
			return fmt.Errorf("product: transaction is required")
		}
		if !columnNamePattern.MatchString(deptColumn) || !columnNamePattern.MatchString(creatorColumn) {
			return fmt.Errorf("product: invalid scope column")
		}
		scope := provider.DataScope(ctx, deptColumn, creatorColumn)
		cast := "BIGINT"
		if database.DriverOf(tx.DB()) == "mysql" || database.DriverOf(tx.DB()) == "mariadb" {
			cast = "SIGNED"
		}
		query := tx.DB().WithContext(ctx).Table(
			"(SELECT CAST(? AS "+cast+") AS "+deptColumn+", CAST(? AS "+cast+") AS "+creatorColumn+") AS candidate",
			m.DeptID, m.CreateBy,
		)
		var visible int64
		if err := scope.Apply(query).Count(&visible).Error; err != nil {
			return err
		}
		if visible != 1 {
			return resource.ErrScopeDenied
		}
		return nil
	}
}

func ensureDeptExists(ctx context.Context, tx resource.Tx, deptID int64) error {
	if tx.DB() == nil {
		return fmt.Errorf("product: transaction is required")
	}
	var found int64
	if err := tx.DB().WithContext(ctx).Table("sys_dept").Where("id = ?", deptID).Count(&found).Error; err != nil {
		return err
	}
	if found != 1 {
		return connect.Errorf(connect.CodeInvalidArgument, "部门不存在")
	}
	return nil
}
