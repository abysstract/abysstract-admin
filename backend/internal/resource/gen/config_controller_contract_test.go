package gen

import (
	"testing"

	testutil "github.com/abysstract/abysstract-admin/backend/internal/testutil"

	"gorm.io/gorm"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// This real generated-package gate ensures the Config controller keeps
// its typed injected repository dependency and that the generated adapter
// covers the full Connect handler interface without Overrides.
var _ adminv1connect.ConfigServiceHandler = (*ConfigServiceAdapter)(nil)

func TestGeneratedConfigControllerAndAdapterRequireTypedDependencies(t *testing.T) {
	baseRepo := repo.New(&gorm.DB{})
	if _, err := NewConfigController(ConfigControllerDeps{Store: testutil.Store(t, baseRepo)}); err == nil {
		t.Fatal("ConfigController accepted nil injected repository")
	}
	controller, err := NewConfigController(ConfigControllerDeps{
		Store:      testutil.Store(t, baseRepo),
		Repository: NewConfigRepository(),
	})
	if err != nil {
		t.Fatalf("NewConfigController: %v", err)
	}
	if _, err := NewConfigServiceAdapter(nil); err == nil {
		t.Fatal("ConfigServiceAdapter accepted nil Config controller")
	}
	if _, err := NewConfigServiceAdapter(controller); err != nil {
		t.Fatalf("NewConfigServiceAdapter: %v", err)
	}
}
