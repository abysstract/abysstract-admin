package gen

import (
	"context"
	"testing"

	"gorm.io/gorm"

	testutil "github.com/abysstract/abysstract-admin/backend/internal/testutil"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

var _ adminv1connect.NoticeServiceHandler = (*NoticeServiceAdapter)(nil)

func TestGeneratedNoticeConstructorsAndAdapterCompileGate(t *testing.T) {
	if _, err := NewNoticeController(NoticeControllerDeps{}); err == nil {
		t.Fatal("Notice controller accepted nil BaseRepo")
	}
	controller, err := NewNoticeController(NoticeControllerDeps{Store: testutil.Store(t, repo.New(&gorm.DB{}))})
	if err != nil {
		t.Fatalf("NewNoticeController: %v", err)
	}
	adapter, err := NewNoticeServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewNoticeServiceAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewNoticeServiceAdapter returned nil adapter")
	}

	// Keep the generated handler path statically tied to the standard Connect
	// signatures; this call is never executed against the nil compile fixture.
	var _ func(context.Context, *adminv1.ListNoticesRequest) (*adminv1.ListNoticesResponse, error) = adapter.ListNotices
}
