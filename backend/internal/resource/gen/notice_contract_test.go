package gen

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestGeneratedNoticeConverterPreservesPresenceAndRejectsNarrowingOverflow(t *testing.T) {
	old := &model.SysNotice{
		ID:            7,
		NoticeTitle:   "old title",
		NoticeType:    1,
		NoticeContent: "old content",
		Remark:        "old remark",
	}

	updatedTitle := "new title"
	updated, columns, err := NoticeUpdateToModel(old, &adminv1.UpdateNoticeRequest{NoticeTitle: &updatedTitle})
	if err != nil {
		t.Fatalf("selective Notice update: %v", err)
	}
	if updated.NoticeTitle != updatedTitle || updated.NoticeContent != old.NoticeContent || updated.NoticeType != old.NoticeType || updated.Remark != old.Remark {
		t.Fatalf("optional Notice update overwrote omitted fields: updated=%+v old=%+v", updated, old)
	}
	if got := columns.Values(); len(got) != 1 || got[0] != NoticeColumnNoticeTitle {
		t.Fatalf("selective Notice columns = %v, want [%q]", got, NoticeColumnNoticeTitle)
	}

	explicitZero := int32(0)
	zero, columns, err := NoticeUpdateToModel(old, &adminv1.UpdateNoticeRequest{NoticeType: &explicitZero})
	if err != nil {
		t.Fatalf("explicit zero Notice update: %v", err)
	}
	if zero.NoticeType != 0 || columns.Len() != 1 || columns.Values()[0] != NoticeColumnNoticeType {
		t.Fatalf("explicit zero Notice update = model=%+v columns=%v, want type 0 and notice_type column", zero, columns.Values())
	}

	overflow := int32(32768)
	if _, _, err := NoticeUpdateToModel(old, &adminv1.UpdateNoticeRequest{NoticeType: &overflow}); err == nil || !strings.Contains(err.Error(), "notice_type") {
		t.Fatalf("Notice update overflow error = %v, want notice_type error", err)
	}
}

func TestGeneratedNoticeValidatorAndEntityRejectUnknownValues(t *testing.T) {
	if err := NoticeValidateCreate(&adminv1.CreateNoticeRequest{NoticeTitle: "title", NoticeContent: "content"}); err != nil {
		t.Fatalf("valid Notice create rejected: %v", err)
	}
	if err := NoticeValidateCreate(&adminv1.CreateNoticeRequest{NoticeTitle: "title", NoticeContent: "content", Remark: strings.Repeat("x", 501)}); err == nil {
		t.Fatal("Notice remark over MaxLen unexpectedly passed")
	}
	zero := int32(0)
	if err := NoticeValidateUpdate(&adminv1.UpdateNoticeRequest{NoticeType: &zero}); err != nil {
		t.Fatalf("Notice update presence validation should leave domain semantics to Hook: %v", err)
	}
	if _, err := NoticeToEntity(&model.SysNotice{Status: 99}); err == nil || !strings.Contains(err.Error(), "未知 model 值") {
		t.Fatalf("unknown Notice status conversion error = %v, want status error", err)
	}
}

func TestGeneratedNoticeColumnPolicyUsesDBNamesAndRejectsDrift(t *testing.T) {
	valid := repo.NewColumnSet(NoticeColumnNoticeTitle, NoticeColumnNoticeType, NoticeColumnNoticeContent, NoticeColumnRemark)
	if err := NoticeValidateUpdateColumns(valid); err != nil {
		t.Fatalf("valid Notice ColumnSet rejected: %v", err)
	}
	for name, columns := range map[string]repo.ColumnSet{
		"empty":          repo.NewColumnSet(),
		"duplicate":      repo.NewColumnSet(NoticeColumnRemark, NoticeColumnRemark),
		"key":            repo.NewColumnSet(repo.Column("id")),
		"server-managed": repo.NewColumnSet(repo.Column("create_time")),
		"unknown":        repo.NewColumnSet(repo.Column("notice_title_json")),
	} {
		if err := NoticeValidateUpdateColumns(columns); err == nil {
			t.Errorf("invalid Notice ColumnSet %s unexpectedly passed", name)
		}
	}
	if got := string(NoticeColumnNoticeTitle); got != "notice_title" {
		t.Fatalf("Notice title DBName = %q, want notice_title", got)
	}
	if got := string(NoticeColumnNoticeContent); got != "notice_content" {
		t.Fatalf("Notice content DBName = %q, want notice_content", got)
	}
}
