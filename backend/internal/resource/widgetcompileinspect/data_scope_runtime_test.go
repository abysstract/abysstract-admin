//go:build integration

package gen

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type denyWidgetScope struct{}

func (denyWidgetScope) DataScope(context.Context, string, string) repo.AccessScope {
	return repo.AccessScope{}
}

type unrestrictedWidgetScope struct{}

func (unrestrictedWidgetScope) DataScope(context.Context, string, string) repo.AccessScope {
	return repo.UnrestrictedAccessScope()
}
func allowWidgetCreate(context.Context, resource.Tx, *schematest.WidgetModel) error { return nil }

func TestGeneratedDataScopeDeniesDeleteBeforeHook(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := db.AutoMigrate(&schematest.WidgetModel{}); err != nil {
		t.Fatal(err)
	}
	r := repo.New(db)
	row := &schematest.WidgetModel{Name: "scope-runtime"}
	if err := NewWidgetRepository().Create(context.Background(), r, row); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = NewWidgetRepository().DeleteByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	})
	beforeUpdate, beforeDelete, beforeBatch := false, false, false
	failCreate := true
	afterCreate := 0
	createErr := errors.New("before create")
	c, err := NewWidgetController(WidgetControllerDeps{
		Store: testutil.Store(t, r), DataScopeProvider: denyWidgetScope{},
		CreateScopeValidator: func(context.Context, resource.Tx, *schematest.WidgetModel) error { return nil },
		Hooks: &resource.HookSet[schematest.WidgetModel]{
			BeforeCreate: func(context.Context, resource.Tx, *schematest.WidgetModel) error {
				if failCreate {
					return createErr
				}
				return nil
			},
			AfterCommitCreate: func(context.Context, schematest.WidgetModel) { afterCreate++ },
			BeforeUpdate: func(context.Context, resource.Tx, *schematest.WidgetModel, *schematest.WidgetModel, repo.ColumnSet) error {
				beforeUpdate = true
				return nil
			},
			BeforeDelete:      func(context.Context, resource.Tx, *schematest.WidgetModel) error { beforeDelete = true; return nil },
			BeforeBatchDelete: func(context.Context, resource.Tx, []*schematest.WidgetModel) error { beforeBatch = true; return nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), CreateScopeValidator: func(context.Context, resource.Tx, *schematest.WidgetModel) error { return nil }}); err == nil {
		t.Fatal("data_scope controller accepted nil DataScopeProvider")
	}
	if _, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: denyWidgetScope{}}); err == nil {
		t.Fatal("data_scope controller accepted nil CreateScopeValidator")
	}
	if _, err := c.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "rollback-create"}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("Create rollback code = %v, want Internal; err=%v", connect.CodeOf(err), err)
	}
	var count int64
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "rollback-create").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed create persisted: count=%d err=%v", count, err)
	}
	failCreate = false
	if _, err := c.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "commit-create"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if afterCreate != 1 {
		t.Fatalf("AfterCommitCreate count = %d, want 1", afterCreate)
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "commit-create").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("successful create missing: count=%d err=%v", count, err)
	}
	deniedCreate, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: denyWidgetScope{}, CreateScopeValidator: func(context.Context, resource.Tx, *schematest.WidgetModel) error { return resource.ErrScopeDenied }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deniedCreate.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "scope-denied-create"}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("scope create code = %v, want PermissionDenied; err=%v", connect.CodeOf(err), err)
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "scope-denied-create").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("scope-denied create persisted: count=%d err=%v", count, err)
	}
	beforeScopeValidator := false
	orderedScopeCreate, err := NewWidgetController(WidgetControllerDeps{
		Store: testutil.Store(t, r), DataScopeProvider: denyWidgetScope{},
		Hooks: &resource.HookSet[schematest.WidgetModel]{BeforeCreate: func(context.Context, resource.Tx, *schematest.WidgetModel) error {
			beforeScopeValidator = true
			return nil
		}},
		CreateScopeValidator: func(_ context.Context, txRepoSession resource.Tx, _ *schematest.WidgetModel) error {
			txRepo := repo.New(txRepoSession.DB())

			if !beforeScopeValidator {
				return errors.New("scope validator ran before BeforeCreate")
			}
			if err := NewWidgetRepository().Create(context.Background(), txRepo, &schematest.WidgetModel{Name: "scope-validator-transaction"}); err != nil {
				return err
			}
			return resource.ErrScopeDenied
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orderedScopeCreate.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "ordered-scope-denied"}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ordered scope denial code = %v, want PermissionDenied; err=%v", connect.CodeOf(err), err)
	}
	if !beforeScopeValidator {
		t.Fatal("BeforeCreate did not run before CreateScopeValidator")
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name IN ?", []string{"ordered-scope-denied", "scope-validator-transaction"}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("scope validator work was not rolled back: count=%d err=%v", count, err)
	}
	internalScopeCreate, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: denyWidgetScope{}, CreateScopeValidator: func(context.Context, resource.Tx, *schematest.WidgetModel) error {
		return errors.New("scope infrastructure failure")
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := internalScopeCreate.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "scope-internal-create"}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("internal scope error code = %v, want Internal; err=%v", connect.CodeOf(err), err)
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "scope-internal-create").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("internal scope failure persisted: count=%d err=%v", count, err)
	}
	updatePhase := "ok"
	beforeUpdateCount, afterUpdateCount, afterCommitUpdateCount := 0, 0, 0
	unrestricted, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{
		BeforeUpdate: func(context.Context, resource.Tx, *schematest.WidgetModel, *schematest.WidgetModel, repo.ColumnSet) error {
			beforeUpdateCount++
			if updatePhase == "before-fail" {
				return errors.New("before update")
			}
			return nil
		},
		AfterUpdate: func(_ context.Context, _ resource.Tx, _ *schematest.WidgetModel, m *schematest.WidgetModel, _ repo.ColumnSet) error {
			afterUpdateCount++
			if updatePhase == "entity-fail" {
				m.UpdatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			return nil
		},
		AfterCommitUpdate: func(context.Context, schematest.WidgetModel) { afterCommitUpdateCount++ },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty update code = %v, want InvalidArgument; err=%v", connect.CodeOf(err), err)
	}
	before, err := NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatal(err)
	}
	same := before.Name
	if _, err := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID, Name: &same}); err != nil {
		t.Fatalf("same update: %v", err)
	}
	after, err := NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatal(err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) || beforeUpdateCount != 0 || afterUpdateCount != 0 || afterCommitUpdateCount != 0 {
		t.Fatalf("same-value update was not a no-op")
	}
	changed := "updated-runtime"
	if _, err := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID, Name: &changed}); err != nil {
		t.Fatalf("changed update: %v", err)
	}
	after, err = NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if err != nil || after.Name != changed || !after.UpdatedAt.After(before.UpdatedAt) || afterCommitUpdateCount != 1 {
		t.Fatalf("changed update wrong: row=%+v err=%v hooks=%d", after, err, afterCommitUpdateCount)
	}
	updatePhase = "before-fail"
	failed := "before-rollback"
	if _, err := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID, Name: &failed}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("before failure code = %v, want Internal", connect.CodeOf(err))
	}
	after, _ = NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if after.Name != changed || afterCommitUpdateCount != 1 {
		t.Fatal("BeforeUpdate failure did not rollback")
	}
	updatePhase = "entity-fail"
	failed = "entity-rollback"
	if _, err := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID, Name: &failed}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("ToEntity failure code = %v, want Internal", connect.CodeOf(err))
	}
	after, _ = NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if after.Name != changed || afterCommitUpdateCount != 1 {
		t.Fatal("ToEntity failure did not rollback")
	}
	deletePhase := ""
	beforeDeleteCount, afterDeleteCount, afterCommitDeleteCount := 0, 0, 0
	deleteSnapshotName := ""
	unrestrictedDelete, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{
		BeforeDelete: func(_ context.Context, _ resource.Tx, old *schematest.WidgetModel) error {
			beforeDeleteCount++
			if deletePhase == "before-fail" {
				return errors.New("before delete")
			}
			if deletePhase == "success" {
				old.Name = "mutated-by-hook"
			}
			return nil
		},
		AfterDelete: func(context.Context, resource.Tx, *schematest.WidgetModel) error {
			afterDeleteCount++
			if deletePhase == "after-fail" {
				return errors.New("after delete")
			}
			return nil
		},
		AfterCommitDelete: func(_ context.Context, snapshot schematest.WidgetModel) {
			afterCommitDeleteCount++
			deleteSnapshotName = snapshot.Name
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	seedDelete := func(name string) *schematest.WidgetModel {
		m := &schematest.WidgetModel{Name: name}
		if err := NewWidgetRepository().Create(context.Background(), r, m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	beforeDeleteRow := seedDelete("before-delete")
	deletePhase = "before-fail"
	if _, err := unrestrictedDelete.DeleteWidget(context.Background(), &testpb.DeleteWidgetRequest{Id: beforeDeleteRow.ID}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("BeforeDelete code = %v, want Internal", connect.CodeOf(err))
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, beforeDeleteRow.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("BeforeDelete failure deleted row: %v", err)
	}
	afterDeleteRow := seedDelete("after-delete")
	deletePhase = "after-fail"
	if _, err := unrestrictedDelete.DeleteWidget(context.Background(), &testpb.DeleteWidgetRequest{Id: afterDeleteRow.ID}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("AfterDelete code = %v, want Internal", connect.CodeOf(err))
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, afterDeleteRow.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("AfterDelete failure deleted row: %v", err)
	}
	successDeleteRow := seedDelete("success-delete")
	deletePhase = "success"
	if _, err := unrestrictedDelete.DeleteWidget(context.Background(), &testpb.DeleteWidgetRequest{Id: successDeleteRow.ID}); err != nil {
		t.Fatalf("successful Delete: %v", err)
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, successDeleteRow.ID, repo.UnrestrictedAccessScope()); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("successful Delete kept row: %v", err)
	}
	if afterCommitDeleteCount != 1 || deleteSnapshotName != "success-delete" {
		t.Fatalf("delete snapshot/AfterCommit wrong: count=%d name=%q", afterCommitDeleteCount, deleteSnapshotName)
	}
	batchPhase := ""
	beforeBatchCount, afterBatchCount, afterCommitBatchCount := 0, 0, 0
	batchSnapshotName := ""
	unrestrictedBatch, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{
		BeforeBatchDelete: func(_ context.Context, _ resource.Tx, rows []*schematest.WidgetModel) error {
			beforeBatchCount++
			if batchPhase == "before-fail" {
				return errors.New("before batch")
			}
			if batchPhase == "success" {
				rows[0].Name = "mutated-batch"
			}
			return nil
		},
		AfterBatchDelete: func(context.Context, resource.Tx, []*schematest.WidgetModel) error {
			afterBatchCount++
			if batchPhase == "after-fail" {
				return errors.New("after batch")
			}
			return nil
		},
		AfterCommitBatchDelete: func(_ context.Context, snapshots []schematest.WidgetModel) {
			afterCommitBatchCount++
			batchSnapshotName = snapshots[0].Name
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	beforeBatchRow := seedDelete("before-batch")
	batchPhase = "before-fail"
	if _, err := unrestrictedBatch.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{beforeBatchRow.ID}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("BeforeBatch code = %v, want Internal", connect.CodeOf(err))
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, beforeBatchRow.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("BeforeBatch failure deleted row: %v", err)
	}
	afterBatchRow := seedDelete("after-batch")
	batchPhase = "after-fail"
	if _, err := unrestrictedBatch.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{afterBatchRow.ID}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("AfterBatch code = %v, want Internal", connect.CodeOf(err))
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, afterBatchRow.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("AfterBatch failure deleted row: %v", err)
	}
	successBatchRow := seedDelete("success-batch")
	batchPhase = "success"
	if _, err := unrestrictedBatch.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{successBatchRow.ID, successBatchRow.ID}}); err != nil {
		t.Fatalf("successful BatchDelete: %v", err)
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, successBatchRow.ID, repo.UnrestrictedAccessScope()); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("successful BatchDelete kept row: %v", err)
	}
	if afterCommitBatchCount != 1 || batchSnapshotName != "success-batch" {
		t.Fatalf("batch snapshot/AfterCommit wrong: count=%d name=%q", afterCommitBatchCount, batchSnapshotName)
	}
	mixedBatchRow := seedDelete("mixed-batch-visible")
	beforeMissingBatchHook := beforeBatchCount
	_, mixedMissingBatchErr := unrestrictedBatch.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{mixedBatchRow.ID, mixedBatchRow.ID + 999999}})
	if connect.CodeOf(mixedMissingBatchErr) != connect.CodeNotFound {
		t.Fatalf("mixed missing BatchDelete code = %v, want NotFound; err=%v", connect.CodeOf(mixedMissingBatchErr), mixedMissingBatchErr)
	}
	if beforeBatchCount != beforeMissingBatchHook {
		t.Fatalf("BeforeBatchDelete ran before all IDs were visible: before=%d after=%d", beforeMissingBatchHook, beforeBatchCount)
	}
	if _, err := NewWidgetRepository().GetByKey(context.Background(), r, mixedBatchRow.ID, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("mixed missing BatchDelete deleted visible row: %v", err)
	}
	listA := &schematest.WidgetModel{Name: "a_b", Enabled: false}
	listB := &schematest.WidgetModel{Name: "axb", Enabled: false}
	listC := &schematest.WidgetModel{Name: "a_b_true", Enabled: true}
	for _, m := range []*schematest.WidgetModel{listA, listB, listC} {
		if err := NewWidgetRepository().Create(context.Background(), r, m); err != nil {
			t.Fatal(err)
		}
	}
	falseValue := false
	listResp, err := unrestricted.ListWidgets(context.Background(), &testpb.ListWidgetsRequest{Page: &adminv1.PageRequest{PageNum: 0, PageSize: 1000}, Keyword: "a_b", Enabled: &falseValue})
	if err != nil {
		t.Fatalf("List escaped/filter: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Name != "a_b" || listResp.Page.GetPageNum() != 1 || listResp.Page.GetPageSize() != 100 {
		t.Fatalf("List semantics wrong: %+v page=%+v", listResp.Items, listResp.Page)
	}
	if _, err := unrestricted.ListWidgets(context.Background(), &testpb.ListWidgetsRequest{Page: &adminv1.PageRequest{}, Orders: []*adminv1.OrderBy{{Field: "not_allowed"}}}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid sort code = %v, want InvalidArgument", connect.CodeOf(err))
	}
	toEntityCreate, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{AfterCreate: func(_ context.Context, _ resource.Tx, m *schematest.WidgetModel) error {
		m.UpdatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := toEntityCreate.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "create-entity-rollback"}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("Create ToEntity code = %v, want Internal", connect.CodeOf(err))
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "create-entity-rollback").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("Create ToEntity failure persisted: count=%d err=%v", count, err)
	}
	panicController, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{AfterCommitCreate: func(context.Context, schematest.WidgetModel) { panic("after commit") }}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := panicController.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "panic-after-commit"}); err != nil {
		t.Fatalf("AfterCommit panic changed Create response: %v", err)
	}
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "panic-after-commit").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("AfterCommit panic rolled back committed create: count=%d err=%v", count, err)
	}
	immutableRow := &schematest.WidgetModel{Name: "immutable-old", Enabled: false}
	if err := NewWidgetRepository().Create(context.Background(), r, immutableRow); err != nil {
		t.Fatal(err)
	}
	immutableController, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: unrestrictedWidgetScope{}, CreateScopeValidator: allowWidgetCreate, Hooks: &resource.HookSet[schematest.WidgetModel]{BeforeUpdate: func(_ context.Context, _ resource.Tx, _ *schematest.WidgetModel, _ *schematest.WidgetModel, columns repo.ColumnSet) error {
		values := columns.Values()
		values[0] = repo.Column("enabled")
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	immutableName := "immutable-new"
	if _, err := immutableController.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: immutableRow.ID, Name: &immutableName}); err != nil {
		t.Fatalf("immutable ColumnSet update: %v", err)
	}
	immutableStored, err := NewWidgetRepository().GetByKey(context.Background(), r, immutableRow.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatal(err)
	}
	if immutableStored.Name != immutableName || immutableStored.Enabled {
		t.Fatalf("Hook mutated update columns: %+v", immutableStored)
	}
	deniedListResp, err := c.ListWidgets(context.Background(), &testpb.ListWidgetsRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("deny-all List returned error: %v", err)
	}
	if len(deniedListResp.Items) != 0 || deniedListResp.Page.GetTotal() != 0 {
		t.Fatalf("zero DataScope leaked List rows: items=%+v page=%+v", deniedListResp.Items, deniedListResp.Page)
	}
	missingID := row.ID + 999999
	_, missingGetErr := unrestricted.GetWidget(context.Background(), &testpb.GetWidgetRequest{Id: missingID})
	_, scopeGetErr := c.GetWidget(context.Background(), &testpb.GetWidgetRequest{Id: row.ID})
	if connect.CodeOf(missingGetErr) != connect.CodeNotFound || connect.CodeOf(scopeGetErr) != connect.CodeNotFound {
		t.Fatalf("Get codes: missing=%v scope=%v", connect.CodeOf(missingGetErr), connect.CodeOf(scopeGetErr))
	}
	if missingGetErr.Error() != scopeGetErr.Error() {
		t.Fatalf("missing and out-of-scope Get leaked different errors: missing=%q scope=%q", missingGetErr, scopeGetErr)
	}
	name := "should-not-write"
	_, missingUpdateErr := unrestricted.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: missingID, Name: &name})
	_, scopeUpdateErr := c.UpdateWidget(context.Background(), &testpb.UpdateWidgetRequest{Id: row.ID, Name: &name})
	if connect.CodeOf(missingUpdateErr) != connect.CodeNotFound || connect.CodeOf(scopeUpdateErr) != connect.CodeNotFound {
		t.Fatalf("Update codes: missing=%v scope=%v", connect.CodeOf(missingUpdateErr), connect.CodeOf(scopeUpdateErr))
	}
	if missingUpdateErr.Error() != scopeUpdateErr.Error() {
		t.Fatalf("missing and out-of-scope Update leaked different errors: missing=%q scope=%q", missingUpdateErr, scopeUpdateErr)
	}
	_, missingDeleteErr := unrestrictedDelete.DeleteWidget(context.Background(), &testpb.DeleteWidgetRequest{Id: missingID})
	_, scopeDeleteErr := c.DeleteWidget(context.Background(), &testpb.DeleteWidgetRequest{Id: row.ID})
	if connect.CodeOf(missingDeleteErr) != connect.CodeNotFound || connect.CodeOf(scopeDeleteErr) != connect.CodeNotFound {
		t.Fatalf("Delete codes: missing=%v scope=%v", connect.CodeOf(missingDeleteErr), connect.CodeOf(scopeDeleteErr))
	}
	if missingDeleteErr.Error() != scopeDeleteErr.Error() {
		t.Fatalf("missing and out-of-scope Delete leaked different errors: missing=%q scope=%q", missingDeleteErr, scopeDeleteErr)
	}
	_, missingBatchErr := unrestrictedBatch.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{missingID}})
	_, scopeBatchErr := c.BatchDeleteWidgets(context.Background(), &testpb.BatchDeleteWidgetsRequest{Ids: []int64{row.ID}})
	if connect.CodeOf(missingBatchErr) != connect.CodeNotFound || connect.CodeOf(scopeBatchErr) != connect.CodeNotFound {
		t.Fatalf("BatchDelete codes: missing=%v scope=%v", connect.CodeOf(missingBatchErr), connect.CodeOf(scopeBatchErr))
	}
	if missingBatchErr.Error() != scopeBatchErr.Error() {
		t.Fatalf("missing and out-of-scope BatchDelete leaked different errors: missing=%q scope=%q", missingBatchErr, scopeBatchErr)
	}
	if beforeUpdate || beforeDelete || beforeBatch {
		t.Fatalf("scope-invisible Hooks ran: update=%v delete=%v batch=%v", beforeUpdate, beforeDelete, beforeBatch)
	}
	stored, err := NewWidgetRepository().GetByKey(context.Background(), r, row.ID, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("row was deleted: %v", err)
	}
	if stored.Name != changed {
		t.Fatalf("row was changed: %q", stored.Name)
	}
}
