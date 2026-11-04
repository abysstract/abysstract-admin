package service

import (
	"context"

	"connectrpc.com/connect/v2"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

const (
	noticeTypeNotification = int16(1)
	noticeTypeAnnouncement = int16(2)
)

// validateNoticeType is the only Notice-specific enum semantic. The wire field
// remains a plain int32 for compatibility with the existing contract; this
// hook rejects values outside the established 1=通知/2=公告 domain.
func validateNoticeType(value int16) error {
	if value != noticeTypeNotification && value != noticeTypeAnnouncement {
		return connect.Errorf(
			connect.CodeInvalidArgument,
			"notice_type 必须为 1（通知）或 2（公告），当前为 %d", value,
		)
	}
	return nil
}

// newNoticeHooks preserves the legacy database/default behavior without
// copying any standard CRUD orchestration into the service layer.
func newNoticeHooks() *resource.HookSet[model.SysNotice] {
	return &resource.HookSet[model.SysNotice]{
		BeforeCreate: func(_ context.Context, _ resource.Tx, notice *model.SysNotice) error {
			if notice.NoticeType == 0 {
				notice.NoticeType = noticeTypeNotification
			}
			if notice.Status == 0 {
				notice.Status = 1
			}
			return validateNoticeType(notice.NoticeType)
		},
		BeforeUpdate: func(_ context.Context, _ resource.Tx, _ *model.SysNotice, notice *model.SysNotice, _ repo.ColumnSet) error {
			return validateNoticeType(notice.NoticeType)
		},
	}
}

// NewGeneratedNoticeService composes the generated Notice Controller and
// ServiceAdapter. Notice-specific defaults/enum semantics remain in hooks;
// standard List/Get/Create/Update/Delete/BatchDelete stay generated.
func NewGeneratedNoticeService(baseRepo *repo.Repo) (adminv1connect.NoticeServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	controller, err := resourcegen.NewNoticeController(resourcegen.NoticeControllerDeps{
		Store: store,
		Hooks: resource.ComposeHooks(newNoticeHooks(), nil),
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewNoticeServiceAdapter(controller)
}
