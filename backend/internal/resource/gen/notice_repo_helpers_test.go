package gen

import (
	"context"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func newNoticeIntegrationController(t *testing.T, base *repo.Repo, hooks *resource.HookSet[model.SysNotice]) *NoticeController {
	t.Helper()
	controller, err := NewNoticeController(NoticeControllerDeps{Store: testutil.Store(t, base), Hooks: hooks})
	if err != nil {
		t.Fatalf("NewNoticeController: %v", err)
	}
	return controller
}

func createIntegrationNotice(t *testing.T, controller *NoticeController, title string) int64 {
	t.Helper()
	response, err := controller.CreateNotice(context.Background(), &adminv1.CreateNoticeRequest{
		NoticeTitle:   title,
		NoticeType:    1,
		NoticeContent: "内容",
		Remark:        "备注",
	})
	if err != nil {
		t.Fatalf("CreateNotice(%q): %v", title, err)
	}
	if response.Item == nil || response.Item.Id == 0 {
		t.Fatalf("CreateNotice(%q) returned no item: %v", title, response.Item)
	}
	return response.Item.Id
}

func noticeIndex(items []*adminv1.Notice, id int64) int {
	for i, item := range items {
		if item.Id == id {
			return i
		}
	}
	return -1
}
