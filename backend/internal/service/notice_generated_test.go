package service

import (
	"context"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestNoticeHooksPreserveDefaultAndEnumBoundary(t *testing.T) {
	hooks := newNoticeHooks()
	create := &model.SysNotice{}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, create); err != nil {
		t.Fatalf("default Notice create rejected: %v", err)
	}
	if create.NoticeType != noticeTypeNotification || create.Status != 1 {
		t.Fatalf("default Notice create = type %d/status %d, want type 1/status 1", create.NoticeType, create.Status)
	}

	for _, value := range []int16{noticeTypeNotification, noticeTypeAnnouncement} {
		if err := hooks.BeforeUpdate(context.Background(), resource.Tx{}, &model.SysNotice{}, &model.SysNotice{NoticeType: value}, repo.NewColumnSet()); err != nil {
			t.Fatalf("valid Notice type %d rejected: %v", value, err)
		}
	}
	for _, value := range []int16{0, 3, -1} {
		err := hooks.BeforeUpdate(context.Background(), resource.Tx{}, &model.SysNotice{}, &model.SysNotice{NoticeType: value}, repo.NewColumnSet())
		if err == nil || !strings.Contains(err.Error(), "notice_type") {
			t.Fatalf("invalid Notice type %d error = %v, want notice_type validation", value, err)
		}
	}
}

func TestNewGeneratedNoticeServiceRejectsNilBaseRepo(t *testing.T) {
	if svc, err := NewGeneratedNoticeService(nil); err == nil || svc != nil {
		t.Fatalf("NewGeneratedNoticeService(nil) = svc=%v err=%v, want nil service and error", svc, err)
	}
}
