//go:build integration

package service

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func TestGeneratedNoticeServiceDatabaseDefaultAndEnumHook(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc, err := NewGeneratedNoticeService(repo.New(gdb))
	if err != nil {
		t.Fatalf("NewGeneratedNoticeService: %v", err)
	}

	created, err := svc.CreateNotice(context.Background(), &adminv1.CreateNoticeRequest{
		NoticeTitle:   "default type",
		NoticeContent: "content",
		// Zero is the legacy omitted/default value and must become 1 in the Hook.
		NoticeType: 0,
	})
	if err != nil {
		t.Fatalf("CreateNotice default type: %v", err)
	}
	if created.Item == nil || created.Item.NoticeType != 1 || created.Item.Status != adminv1.Status_STATUS_ENABLED {
		t.Fatalf("default Notice response = %v, want notice_type=1/status=enabled", created.Item)
	}

	for _, invalid := range []int32{3, -1} {
		_, err := svc.CreateNotice(context.Background(), &adminv1.CreateNoticeRequest{
			NoticeTitle:   "invalid type",
			NoticeType:    invalid,
			NoticeContent: "content",
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateNotice notice_type=%d code = %v, want InvalidArgument; err=%v", invalid, connect.CodeOf(err), err)
		}
	}

	id := created.Item.Id
	validUpdateType := int32(2)
	updated, err := svc.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{
		Id:         id,
		NoticeType: &validUpdateType,
	})
	if err != nil {
		t.Fatalf("UpdateNotice valid type: %v", err)
	}
	if updated.Item.GetNoticeType() != 2 {
		t.Fatalf("UpdateNotice type = %d, want 2", updated.Item.GetNoticeType())
	}

	for _, invalidUpdateType := range []int32{0, 3} {
		if _, err := svc.UpdateNotice(context.Background(), &adminv1.UpdateNoticeRequest{
			Id:         id,
			NoticeType: &invalidUpdateType,
		}); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("UpdateNotice notice_type=%d code = %v, want InvalidArgument; err=%v", invalidUpdateType, connect.CodeOf(err), err)
		}
	}
	got, err := svc.GetNotice(context.Background(), &adminv1.GetNoticeRequest{Id: id})
	if err != nil {
		t.Fatalf("GetNotice after rejected type update: %v", err)
	}
	if got.Item.GetNoticeType() != 2 {
		t.Fatalf("rejected notice_type update changed persisted value to %d", got.Item.GetNoticeType())
	}
}
