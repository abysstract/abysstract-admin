//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"

	resource "github.com/abysstract/abysstract-admin/backend/resource"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"

	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func openDeptIntegration(t *testing.T) (*repo.Repo, func()) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.New(gdb), func() {}
}

func TestGeneratedDeptDatabaseCRUDPresenceAndSoftDelete(t *testing.T) {
	base, closeDB := openDeptIntegration(t)
	defer closeDB()
	handler := newDeptIntegrationHandler(t, base)
	ctx := context.Background()

	root := createIntegrationDept(t, handler, 0, "root")
	secondRoot := createIntegrationDept(t, handler, 0, "second")
	child := createIntegrationDept(t, handler, root.Id, "child")
	grandchild := createIntegrationDept(t, handler, child.Id, "grandchild")

	rollbackHooks := newDeptHooks()
	rollbackHooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysDept, *model.SysDept, repo.ColumnSet) error {
		return errors.New("forced Dept update rollback")
	}
	rollbackHooks.AfterDelete = func(context.Context, resource.Tx, *model.SysDept) error {
		return errors.New("forced Dept delete rollback")
	}
	rollbackHooks.AfterBatchDelete = func(context.Context, resource.Tx, []*model.SysDept) error {
		return errors.New("forced Dept batch rollback")
	}
	rollbackController, err := resourcegen.NewDeptController(resourcegen.DeptControllerDeps{
		Store:      testutil.Store(t, base),
		Hooks:      rollbackHooks,
		Repository: newDeptRepository(),
	})
	if err != nil {
		t.Fatalf("NewDeptController rollback fixture: %v", err)
	}
	rollbackParent := secondRoot.Id
	if _, err := rollbackController.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: child.Id, ParentId: &rollbackParent}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Dept update rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var rollbackChild, rollbackGrandchild model.SysDept
	if err := base.DB().Where("id = ?", child.Id).First(&rollbackChild).Error; err != nil {
		t.Fatalf("load child after update rollback: %v", err)
	}
	if err := base.DB().Where("id = ?", grandchild.Id).First(&rollbackGrandchild).Error; err != nil {
		t.Fatalf("load grandchild after update rollback: %v", err)
	}
	if rollbackChild.ParentID != root.Id || rollbackChild.Ancestors != "0,"+stringInt64(root.Id) || rollbackGrandchild.Ancestors != "0,"+stringInt64(root.Id)+","+stringInt64(child.Id) {
		t.Fatalf("Dept update rollback leaked ancestor changes: child=%+v grandchild=%+v", rollbackChild, rollbackGrandchild)
	}

	flat, err := handler.ListDeptRows(ctx, &adminv1.ListDeptRowsRequest{
		Page:    &adminv1.PageRequest{PageNum: 1, PageSize: 10},
		Keyword: "child",
	})
	if err != nil {
		t.Fatalf("ListDeptRows keyword: %v", err)
	}
	if flat.Page == nil || flat.Page.Total != 2 || len(flat.Items) != 2 {
		t.Fatalf("ListDeptRows keyword = items=%v page=%v, want child+grandchild flat rows", flat.Items, flat.Page)
	}
	before := model.SysDept{}
	if err := base.DB().Where("id = ?", child.Id).First(&before).Error; err != nil {
		t.Fatalf("load child before update: %v", err)
	}
	newName := "child-renamed"
	updated, err := handler.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: child.Id, DeptName: &newName})
	if err != nil {
		t.Fatalf("selective UpdateDept: %v", err)
	}
	if updated.Dept == nil || updated.Dept.DeptName != newName || updated.Dept.ParentId != before.ParentID {
		t.Fatalf("selective UpdateDept response = %#v, omitted fields were not preserved", updated.Dept)
	}
	var after model.SysDept
	if err := base.DB().Where("id = ?", child.Id).First(&after).Error; err != nil {
		t.Fatalf("load child after update: %v", err)
	}
	if after.DeptName != newName || after.ParentID != before.ParentID || !after.UpdateTime.After(before.UpdateTime) {
		t.Fatalf("selective Dept update persisted incorrectly: before=%+v after=%+v", before, after)
	}

	sameName := newName
	sameAt := after.UpdateTime
	same, err := handler.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: child.Id, DeptName: &sameName})
	if err != nil {
		t.Fatalf("same-value UpdateDept: %v", err)
	}
	if same.Dept == nil || !same.Dept.UpdateTime.AsTime().Equal(sameAt) {
		t.Fatalf("same-value UpdateDept changed UpdatedAt: got=%v want=%s", same.Dept, sameAt)
	}

	if _, err := handler.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: child.Id}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty Dept patch code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}

	newParent := secondRoot.Id
	moved, err := handler.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: child.Id, ParentId: &newParent})
	if err != nil {
		t.Fatalf("move Dept subtree: %v", err)
	}
	if moved.Dept == nil || moved.Dept.ParentId != secondRoot.Id {
		t.Fatalf("moved Dept response = %#v", moved.Dept)
	}
	var movedChild, movedGrandchild model.SysDept
	if err := base.DB().Where("id = ?", child.Id).First(&movedChild).Error; err != nil {
		t.Fatalf("load moved child: %v", err)
	}
	if err := base.DB().Where("id = ?", grandchild.Id).First(&movedGrandchild).Error; err != nil {
		t.Fatalf("load moved grandchild: %v", err)
	}
	if movedChild.Ancestors != "0,"+stringInt64(secondRoot.Id) || movedGrandchild.Ancestors != "0,"+stringInt64(secondRoot.Id)+","+stringInt64(child.Id) {
		t.Fatalf("ancestor cascade = child=%q grandchild=%q", movedChild.Ancestors, movedGrandchild.Ancestors)
	}

	cycleParent := grandchild.Id
	if _, err := handler.UpdateDept(ctx, &adminv1.UpdateDeptRequest{Id: secondRoot.Id, ParentId: &cycleParent}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Dept cycle code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}

	if _, err := handler.DeleteDept(ctx, &adminv1.DeleteDeptRequest{Id: secondRoot.Id}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Dept parent delete code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	var stillThere model.SysDept
	if err := base.DB().Where("id = ? AND del_flag = 0", secondRoot.Id).First(&stillThere).Error; err != nil {
		t.Fatalf("failed child-protected delete changed row: %v", err)
	}

	if _, err := rollbackController.DeleteDept(ctx, &adminv1.DeleteDeptRequest{Id: grandchild.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Dept delete rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var activeGrandchild model.SysDept
	if err := base.DB().Where("id = ? AND del_flag = 0", grandchild.Id).First(&activeGrandchild).Error; err != nil {
		t.Fatalf("Dept delete rollback leaked soft delete: %v", err)
	}
	if _, err := handler.DeleteDept(ctx, &adminv1.DeleteDeptRequest{Id: grandchild.Id}); err != nil {
		t.Fatalf("DeleteDept leaf: %v", err)
	}
	var deleted model.SysDept
	if err := base.DB().Where("id = ?", grandchild.Id).First(&deleted).Error; err != nil {
		t.Fatalf("load soft-deleted grandchild: %v", err)
	}
	if deleted.DelFlag != 1 {
		t.Fatalf("Dept Delete del_flag = %d, want 1", deleted.DelFlag)
	}
	if _, err := handler.GetDept(ctx, &adminv1.GetDeptRequest{Id: grandchild.Id}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Get deleted Dept code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}

	batchOne := createIntegrationDept(t, handler, 0, "batch-one")
	batchTwo := createIntegrationDept(t, handler, 0, "batch-two")
	if _, err := rollbackController.BatchDeleteDepts(ctx, &adminv1.BatchDeleteDeptsRequest{Ids: []int64{batchOne.Id, batchTwo.Id}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Dept batch rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var activeBatch int64
	if err := base.DB().Model(&model.SysDept{}).Where("id IN ? AND del_flag = 0", []int64{batchOne.Id, batchTwo.Id}).Count(&activeBatch).Error; err != nil {
		t.Fatalf("count batch rows after rollback: %v", err)
	}
	if activeBatch != 2 {
		t.Fatalf("Dept batch rollback active count = %d, want 2", activeBatch)
	}
	if _, err := handler.BatchDeleteDepts(ctx, &adminv1.BatchDeleteDeptsRequest{Ids: []int64{batchOne.Id, batchTwo.Id, batchOne.Id}}); err != nil {
		t.Fatalf("BatchDeleteDepts: %v", err)
	}
	var deletedBatch []model.SysDept
	if err := base.DB().Where("id IN ?", []int64{batchOne.Id, batchTwo.Id}).Find(&deletedBatch).Error; err != nil {
		t.Fatalf("load batch-deleted Depts: %v", err)
	}
	if len(deletedBatch) != 2 || deletedBatch[0].DelFlag != 1 || deletedBatch[1].DelFlag != 1 {
		t.Fatalf("BatchDeleteDepts rows = %#v, want two soft-deleted rows", deletedBatch)
	}
	if _, err := handler.DeleteDept(ctx, &adminv1.DeleteDeptRequest{Id: 999999999}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Delete missing Dept code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	if _, err := handler.BatchDeleteDepts(ctx, &adminv1.BatchDeleteDeptsRequest{Ids: []int64{999999999}}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("BatchDelete missing Dept code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}

	status := adminv1.Status_STATUS_ENABLED
	filtered, err := handler.ListDeptRows(ctx, &adminv1.ListDeptRowsRequest{
		Page:   &adminv1.PageRequest{PageNum: 1, PageSize: 100},
		Status: &status,
	})
	if err != nil {
		t.Fatalf("ListDeptRows status filter: %v", err)
	}
	for _, item := range filtered.Items {
		if item.Status != adminv1.Status_STATUS_ENABLED {
			t.Fatalf("status filter returned %v: %#v", item.Status, item)
		}
	}

	if err := base.DB().Model(&model.SysDept{}).Where("id = ?", root.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("seed explicit-zero ListDeptRows status: %v", err)
	}
	unspecified := adminv1.Status_STATUS_UNSPECIFIED
	zeroFiltered, err := handler.ListDeptRows(ctx, &adminv1.ListDeptRowsRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}, Keyword: "root", Status: &unspecified,
	})
	if err != nil || zeroFiltered.Page == nil || zeroFiltered.Page.Total != 1 || len(zeroFiltered.Items) != 1 || zeroFiltered.Items[0].Id != root.Id {
		t.Fatalf("ListDeptRows explicit STATUS_UNSPECIFIED err=%v items=%v page=%v, want status=0 row", err, zeroFiltered.Items, zeroFiltered.Page)
	}

	// A malformed persisted enum must not be silently narrowed to a valid wire value.
	if err := base.DB().Model(&model.SysDept{}).Where("id = ?", root.Id).Update("status", 99).Error; err != nil {
		t.Fatalf("seed invalid Dept status: %v", err)
	}
	if _, err := handler.GetDept(ctx, &adminv1.GetDeptRequest{Id: root.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unknown Dept status Get code = %v err=%v, want Internal conversion failure", connect.CodeOf(err), err)
	}
}
