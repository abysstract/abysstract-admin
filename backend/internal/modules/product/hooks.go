package product

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/identity"
	gen "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/resource"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var (
	errUnauthenticated = connect.NewError(connect.CodeUnauthenticated, "未认证")
	productNoPattern   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
)

// CommitStats records AfterCommit executions for tests. It is not a delivery mechanism.
type CommitStats struct {
	Creates atomic.Int64
	Updates atomic.Int64
	Deletes atomic.Int64
	Batches atomic.Int64
}

type hookOptions struct {
	writeEvent EventWriter
	observe    *CommitStats
}

func newProductHooks(opts hookOptions) *resource.HookSet[model.ProductItem] {
	writer := opts.writeEvent
	if writer == nil {
		writer = writeProductEvent
	}
	hooks := &resource.HookSet[model.ProductItem]{
		BeforeCreate: beforeCreate,
		BeforeUpdate: beforeUpdate,
		AfterCreate:  writeEventFor(writer, eventCreate),
		AfterUpdate: func(ctx context.Context, tx resource.Tx, _ *model.ProductItem, m *model.ProductItem, _ resource.ColumnSet) error {
			return writeEventFor(writer, eventUpdate)(ctx, tx, m)
		},
		AfterDelete:      writeEventFor(writer, eventDelete),
		AfterBatchDelete: writeEventsFor(writer, eventDelete),
	}
	if opts.observe != nil {
		stats := opts.observe
		hooks.AfterCommitCreate = func(context.Context, model.ProductItem) { stats.Creates.Add(1) }
		hooks.AfterCommitUpdate = func(context.Context, model.ProductItem) { stats.Updates.Add(1) }
		hooks.AfterCommitDelete = func(context.Context, model.ProductItem) { stats.Deletes.Add(1) }
		hooks.AfterCommitBatchDelete = func(context.Context, []model.ProductItem) { stats.Batches.Add(1) }
	}
	return hooks
}

func beforeCreate(ctx context.Context, tx resource.Tx, m *model.ProductItem) error {
	if m == nil {
		return fmt.Errorf("product: model is required")
	}
	normalized, err := normalizeProductNo(m.ProductNo)
	if err != nil {
		return err
	}
	m.ProductNo = normalized
	name, err := normalizeName(m.Name)
	if err != nil {
		return err
	}
	m.Name = name
	if err := validatePrice(m.PriceCents); err != nil {
		return err
	}
	actor, ok := identity.FromContext(ctx)
	if !ok || actor.UserID == 0 {
		return errUnauthenticated
	}
	m.CreateBy = actor.UserID
	return ensureDeptExists(ctx, tx, m.DeptID)
}

func beforeUpdate(ctx context.Context, _ resource.Tx, _ *model.ProductItem, m *model.ProductItem, columns resource.ColumnSet) error {
	if m == nil {
		return fmt.Errorf("product: model is required")
	}
	if columns.Contains(gen.ProductColumnName) {
		name, err := normalizeName(m.Name)
		if err != nil {
			return err
		}
		m.Name = name
	}
	if columns.Contains(gen.ProductColumnPriceCents) {
		if err := validatePrice(m.PriceCents); err != nil {
			return err
		}
	}
	return nil
}

func normalizeProductNo(raw string) (string, error) {
	normalized := identity.ASCIICaseKey(raw)
	if !productNoPattern.MatchString(normalized) {
		return "", connect.Errorf(connect.CodeInvalidArgument, "商品编号须为 1–64 位 ASCII 字母、数字或连字符")
	}
	return normalized, nil
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", connect.Errorf(connect.CodeInvalidArgument, "商品名称不能为空")
	}
	if utf8.RuneCountInString(name) > 100 {
		return "", connect.Errorf(connect.CodeInvalidArgument, "商品名称长度不能超过 100")
	}
	return name, nil
}

func validatePrice(cents int64) error {
	if cents < 0 {
		return connect.Errorf(connect.CodeInvalidArgument, "价格不能为负")
	}
	return nil
}
