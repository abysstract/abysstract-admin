//go:build integration

package gen

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func openNoticeIntegration(t *testing.T) (*repo.Repo, func()) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.New(gdb), func() {}
}

func TestGeneratedNoticeRepositoryDatabaseCRUDAndPresence(t *testing.T) {
	base, closeDB := openNoticeIntegration(t)
	defer closeDB()
	controller := newNoticeIntegrationController(t, base, nil)

	firstID := createIntegrationNotice(t, controller, "first notice")
	secondID := createIntegrationNotice(t, controller, "second notice")
	thirdID := createIntegrationNotice(t, controller, "third notice")

	defaultList, err := controller.ListNotices(context.Background(), &adminv1.ListNoticesRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("ListNotices default order: %v", err)
	}
	if defaultList.Page == nil || len(defaultList.Items) < 3 {
		t.Fatalf("ListNotices default order = items=%v page=%v, want resource key ascending", defaultList.Items, defaultList.Page)
	}
	for i := 1; i < len(defaultList.Items); i++ {
		if defaultList.Items[i-1].Id >= defaultList.Items[i].Id {
			t.Fatalf("ListNotices default order = items=%v, want strictly ascending resource keys", defaultList.Items)
		}
	}
	firstIndex, secondIndex, thirdIndex := noticeIndex(defaultList.Items, firstID), noticeIndex(defaultList.Items, secondID), noticeIndex(defaultList.Items, thirdID)
	if firstIndex < 0 || secondIndex < 0 || thirdIndex < 0 || !(firstIndex < secondIndex && secondIndex < thirdIndex) {
		t.Fatalf("ListNotices default order = items=%v, want created notices in ID ascending order", defaultList.Items)
	}

	list, err := controller.ListNotices(context.Background(), &adminv1.ListNoticesRequest{
		Page:    &adminv1.PageRequest{PageNum: 1, PageSize: 10},
		Keyword: "first",
	})
	if err != nil {
		t.Fatalf("ListNotices keyword: %v", err)
	}
	if list.Page == nil || list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].Id != firstID {
		t.Fatalf("ListNotices keyword result = items=%v page=%v, want first notice only", list.Items, list.Page)
	}

	before := model.SysNotice{}
	if err := base.DB().Where("id = ?", firstID).First(&before).Error; err != nil {
		t.Fatalf("load first before update: %v", err)
	}
	title := "first notice updated"
	updated, err := controller.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{
		Id:          firstID,
		NoticeTitle: &title,
	})
	if err != nil {
		t.Fatalf("selective title UpdateNotice: %v", err)
	}
	if updated.Item.GetNoticeTitle() != title || updated.Item.GetNoticeContent() != before.NoticeContent || updated.Item.GetRemark() != before.Remark {
		t.Fatalf("selective update response lost untouched values: %v", updated.Item)
	}
	var after model.SysNotice
	if err := base.DB().Where("id = ?", firstID).First(&after).Error; err != nil {
		t.Fatalf("load first after update: %v", err)
	}
	if after.NoticeTitle != title || after.NoticeContent != before.NoticeContent || after.Remark != before.Remark {
		t.Fatalf("selective update persisted unexpected values: before=%+v after=%+v", before, after)
	}
	if !after.UpdateTime.After(before.UpdateTime) {
		t.Fatalf("selective update did not advance UpdatedAt: before=%s after=%s", before.UpdateTime, after.UpdateTime)
	}

	emptyRemark := ""
	explicitEmpty, err := controller.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{
		Id:     firstID,
		Remark: &emptyRemark,
	})
	if err != nil {
		t.Fatalf("explicit empty Remark UpdateNotice: %v", err)
	}
	if explicitEmpty.Item.GetRemark() != "" {
		t.Fatalf("explicit empty Remark response = %q, want empty", explicitEmpty.Item.GetRemark())
	}
	var afterExplicitEmpty model.SysNotice
	if err := base.DB().Where("id = ?", firstID).First(&afterExplicitEmpty).Error; err != nil {
		t.Fatalf("load first after explicit empty patch: %v", err)
	}
	if afterExplicitEmpty.Remark != "" || !afterExplicitEmpty.UpdateTime.After(after.UpdateTime) {
		t.Fatalf("explicit empty patch persisted incorrectly: after=%+v", afterExplicitEmpty)
	}

	unchangedAt := afterExplicitEmpty.UpdateTime
	sameRemark := afterExplicitEmpty.Remark
	same, err := controller.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{
		Id:     firstID,
		Remark: &sameRemark,
	})
	if err != nil {
		t.Fatalf("same-value UpdateNotice: %v", err)
	}
	if !same.Item.GetUpdateTime().AsTime().Equal(unchangedAt) {
		t.Fatalf("same-value patch changed UpdatedAt: got=%s want=%s", same.Item.GetUpdateTime().AsTime(), unchangedAt)
	}
	var afterSame model.SysNotice
	if err := base.DB().Where("id = ?", firstID).First(&afterSame).Error; err != nil {
		t.Fatalf("load first after same-value patch: %v", err)
	}
	if !afterSame.UpdateTime.Equal(unchangedAt) {
		t.Fatalf("same-value patch persisted an audit update: got=%s want=%s", afterSame.UpdateTime, unchangedAt)
	}

	if _, err := controller.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{Id: firstID}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty Notice patch code = %v, want InvalidArgument (err=%v)", connect.CodeOf(err), err)
	}
	var afterEmpty model.SysNotice
	if err := base.DB().Where("id = ?", firstID).First(&afterEmpty).Error; err != nil {
		t.Fatalf("load first after empty patch: %v", err)
	}
	if !afterEmpty.UpdateTime.Equal(unchangedAt) {
		t.Fatalf("empty patch changed UpdatedAt: got=%s want=%s", afterEmpty.UpdateTime, unchangedAt)
	}

	rollbackCalled := 0
	rollbackController := newNoticeIntegrationController(t, base, &resource.HookSet[model.SysNotice]{
		BeforeUpdate: func(context.Context, resource.Tx, *model.SysNotice, *model.SysNotice, repo.ColumnSet) error {
			rollbackCalled++
			return errors.New("notice update rollback")
		},
	})
	rollbackTitle := "must rollback"
	if _, err := rollbackController.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{Id: secondID, NoticeTitle: &rollbackTitle}); err == nil {
		t.Fatal("failing Notice BeforeUpdate unexpectedly succeeded")
	}
	if rollbackCalled != 1 {
		t.Fatalf("Notice BeforeUpdate calls = %d, want 1", rollbackCalled)
	}
	var rolledBack model.SysNotice
	if err := base.DB().Where("id = ?", secondID).First(&rolledBack).Error; err != nil {
		t.Fatalf("load rolled back notice: %v", err)
	}
	if rolledBack.NoticeTitle == rollbackTitle {
		t.Fatal("Notice BeforeUpdate failure did not rollback the title")
	}

	toEntityRollbackTitle := "to entity rollback"
	toEntityRollbackController := newNoticeIntegrationController(t, base, &resource.HookSet[model.SysNotice]{
		AfterCreate: func(_ context.Context, _ resource.Tx, notice *model.SysNotice) error {
			notice.Status = 99
			return nil
		},
	})
	if _, err := toEntityRollbackController.CreateNotice(context.Background(), &adminv1.CreateNoticeRequest{
		NoticeTitle:   toEntityRollbackTitle,
		NoticeType:    1,
		NoticeContent: "内容",
	}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("ToEntity failure code = %v, want Internal (err=%v)", connect.CodeOf(err), err)
	}
	var rollbackCount int64
	if err := base.DB().Model(&model.SysNotice{}).Where("notice_title = ?", toEntityRollbackTitle).Count(&rollbackCount).Error; err != nil {
		t.Fatalf("count ToEntity rollback row: %v", err)
	}
	if rollbackCount != 0 {
		t.Fatalf("ToEntity failure committed %d Notice rows", rollbackCount)
	}

	if _, err := controller.DeleteNotice(context.Background(), &adminv1.DeleteNoticeRequest{Id: 999999999}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Delete missing Notice code = %v, want NotFound (err=%v)", connect.CodeOf(err), err)
	}
	if _, err := controller.BatchDeleteNotices(context.Background(), &adminv1.BatchDeleteNoticesRequest{Ids: []int64{firstID, secondID, secondID}}); err != nil {
		t.Fatalf("BatchDeleteNotices existing IDs: %v", err)
	}
	if _, err := controller.GetNotice(context.Background(), &adminv1.GetNoticeRequest{Id: firstID}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Get deleted Notice code = %v, want NotFound (err=%v)", connect.CodeOf(err), err)
	}
	if _, err := controller.BatchDeleteNotices(context.Background(), &adminv1.BatchDeleteNoticesRequest{Ids: []int64{thirdID, 999999999}}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("BatchDelete mixed missing IDs code = %v, want NotFound (err=%v)", connect.CodeOf(err), err)
	}
	if _, err := controller.GetNotice(context.Background(), &adminv1.GetNoticeRequest{Id: thirdID}); err != nil {
		t.Fatalf("BatchDelete mixed missing IDs should rollback third notice: %v", err)
	}

	// The generated converter must reject a model→entity enum drift rather than
	// leaking an unknown Status value to the wire.
	if err := base.DB().Model(&model.SysNotice{}).Where("id = ?", thirdID).Update("status", 99).Error; err != nil {
		t.Fatalf("inject invalid status for converter gate: %v", err)
	}
	if _, err := controller.GetNotice(context.Background(), &adminv1.GetNoticeRequest{Id: thirdID}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unknown Status Get code = %v, want Internal (err=%v)", connect.CodeOf(err), err)
	}

}
