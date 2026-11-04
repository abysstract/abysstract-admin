package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// ListDeptRows is the Dept-specific injected Repository list path. The
// generated query has already validated filters and sort columns; this method
// adds the persisted soft-delete predicate and keeps tree state out of the
// generic Resource repository.
func (r *Repo) ListDeptRows(
	ctx context.Context,
	page Page,
	keyword string,
	parentID *int64,
	status *int16,
	orders []Order,
	access AccessScope,
) ([]*model.SysDept, int64, error) {
	page.Num, page.Size = normalizePage(page.Num, page.Size)
	q := access.Apply(r.db.WithContext(ctx)).Model(&model.SysDept{}).Where("del_flag = 0")
	if search := Search([]Column{"dept_name", "leader", "phone", "email"}, keyword); !search.IsNoop() {
		q = search.Apply(q)
	}
	if parentID != nil {
		q = Equal(Column("parent_id"), *parentID).Apply(q)
	}
	if status != nil {
		q = Equal(Column("status"), *status).Apply(q)
	}
	for _, order := range StableOrders(orders, Column("id")) {
		q = OrderClause(order).Apply(q)
	}

	var total int64
	if err := wrapDBErr(q.Count(&total).Error); err != nil {
		return nil, 0, err
	}
	var list []*model.SysDept
	if err := wrapDBErr(q.Offset((page.Num - 1) * page.Size).Limit(page.Size).Find(&list).Error); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetDeptResource loads only active rows for generated standard CRUD. Deleted
// rows deliberately collapse to ErrNotFound to avoid existence disclosure.
func (r *Repo) GetDeptResource(ctx context.Context, id int64, access AccessScope) (*model.SysDept, error) {
	var d model.SysDept
	if err := access.Apply(r.db.WithContext(ctx)).Where("id = ? AND del_flag = 0", id).First(&d).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &d, nil
}

// GetDeptResourceForUpdate is the transaction-scoped active-row lock used by
// the injected Repository decorator.
func (r *Repo) GetDeptResourceForUpdate(ctx context.Context, id int64, access AccessScope) (*model.SysDept, error) {
	var d model.SysDept
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND del_flag = 0", id).First(&d).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &d, nil
}

// GetManyDeptResourcesForUpdate locks active rows for BatchDelete. The
// generated Controller compares the returned count with distinct request IDs,
// giving missing/deleted IDs the same NotFound response.
func (r *Repo) GetManyDeptResourcesForUpdate(ctx context.Context, ids []int64, access AccessScope) ([]*model.SysDept, error) {
	var list []*model.SysDept
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ? AND del_flag = 0", ids).Find(&list).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return list, nil
}

// PrepareDeptCreate retains the existing status default and computes the
// materialized ancestors path before generated Repository.Create executes.
func (r *Repo) PrepareDeptCreate(ctx context.Context, d *model.SysDept) error {
	if d.Status == 0 {
		d.Status = 1
	}
	ancestors, err := r.deptAncestors(ctx, d.ParentID)
	if err != nil {
		return err
	}
	d.Ancestors = ancestors
	return nil
}

// ValidateDeptParent rejects self-parenting and moving below a descendant.
func (r *Repo) ValidateDeptParent(ctx context.Context, id, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == id {
		return ErrCycle
	}
	parent, err := r.GetDeptResourceForUpdate(ctx, parentID, UnrestrictedAccessScope())
	if err != nil {
		return err
	}
	if containsID(parent.Ancestors, id) {
		return ErrCycle
	}
	return nil
}

// MoveDeptSubtree updates the current node and all descendants in the same
// transaction as the generated Update operation. The caller already owns the
// controller transaction; no nested transaction is opened here.
func (r *Repo) MoveDeptSubtree(ctx context.Context, id, parentID int64) error {
	if err := r.ValidateDeptParent(ctx, id, parentID); err != nil {
		return err
	}
	old, err := r.GetDeptResourceForUpdate(ctx, id, UnrestrictedAccessScope())
	if err != nil {
		return err
	}
	newAncestors, err := r.deptAncestors(ctx, parentID)
	if err != nil {
		return err
	}
	if old.Ancestors == newAncestors {
		return nil
	}

	if err := wrapDBErr(r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("id = ? AND del_flag = 0", id).
		Updates(map[string]any{"ancestors": newAncestors}).Error); err != nil {
		return err
	}

	oldPrefix := fmt.Sprintf("%s,%d", old.Ancestors, id)
	newPrefix := fmt.Sprintf("%s,%d", newAncestors, id)
	var descendants []model.SysDept
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("del_flag = 0 AND id <> ?", id).Where("ancestors = ? OR ancestors LIKE ?", oldPrefix, oldPrefix+",%").Order("id").Find(&descendants).Error; err != nil {
		return wrapDBErr(err)
	}
	now := NormalizeTimestamp(time.Now())
	for _, row := range descendants {
		ancestors := newPrefix + strings.TrimPrefix(row.Ancestors, oldPrefix)
		if err := r.db.WithContext(ctx).Model(&model.SysDept{}).Where("id = ?", row.ID).
			Updates(map[string]any{"ancestors": ancestors, "update_time": now}).Error; err != nil {
			return wrapDBErr(err)
		}
	}
	return nil
}

// ValidateDeptDelete is the specialized child-protection Hook boundary.
func (r *Repo) ValidateDeptDelete(ctx context.Context, id int64) error {
	var children int64
	if err := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("parent_id = ? AND del_flag = 0", id).Count(&children).Error; err != nil {
		return wrapDBErr(err)
	}
	if children > 0 {
		return ErrHasChildren
	}
	return nil
}

// SoftDeleteDept marks one active leaf deleted. It is intentionally separate
// from the generic Repository's hard-delete adapter because sys_dept's
// published compatibility behavior is del_flag-based soft deletion.
func (r *Repo) SoftDeleteDept(ctx context.Context, id int64, access AccessScope) error {
	if err := r.ValidateDeptDelete(ctx, id); err != nil {
		return err
	}
	result := access.Apply(r.db.WithContext(ctx)).Model(&model.SysDept{}).
		Where("id = ? AND del_flag = 0", id).
		Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
	if err := wrapDBErr(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftBatchDeleteDepts applies an all-or-nothing soft delete after the
// generated Controller has locked rows and run BeforeBatchDelete hooks.
func (r *Repo) SoftBatchDeleteDepts(ctx context.Context, ids []int64, access AccessScope) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("dept: BatchDelete ID 集合不能为空")
	}
	result := access.Apply(r.db.WithContext(ctx)).Model(&model.SysDept{}).
		Where("id IN ? AND del_flag = 0", ids).
		Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
	if err := wrapDBErr(result.Error); err != nil {
		return 0, err
	}
	if result.RowsAffected != int64(len(ids)) {
		return result.RowsAffected, ErrNotFound
	}
	return result.RowsAffected, nil
}

func (r *Repo) deptAncestors(ctx context.Context, parentID int64) (string, error) {
	if parentID == 0 {
		return "0", nil
	}
	parent, err := r.GetDeptResourceForUpdate(ctx, parentID, UnrestrictedAccessScope())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID), nil
}

func containsID(ancestors string, id int64) bool {
	target := fmt.Sprintf("%d", id)
	for _, part := range strings.Split(ancestors, ",") {
		if strings.TrimSpace(part) == target {
			return true
		}
	}
	return false
}
