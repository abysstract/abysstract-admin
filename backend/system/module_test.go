package system

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestNewModuleDeclaresThirteenServicesAndAllMethods(t *testing.T) {
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "system" {
		t.Fatalf("name = %q", def.Name)
	}
	if def.Create == nil {
		t.Fatal("Create is nil")
	}
	if def.Permissions == nil {
		t.Fatal("Permissions is nil")
	}
	want := []string{
		"abysstract.admin.v1.AuthService",
		"abysstract.admin.v1.UserService",
		"abysstract.admin.v1.RoleService",
		"abysstract.admin.v1.MenuService",
		"abysstract.admin.v1.DeptService",
		"abysstract.admin.v1.DictService",
		"abysstract.admin.v1.ConfigService",
		"abysstract.admin.v1.NoticeService",
		"abysstract.admin.v1.LogService",
		"abysstract.admin.v1.UploadService",
		"abysstract.admin.v1.CommonExportService",
		"abysstract.admin.v1.ProfileService",
		"abysstract.admin.v1.DashboardService",
	}
	if len(def.Services) != len(want) {
		t.Fatalf("services = %d, want %d", len(def.Services), len(want))
	}
	methodCount := 0
	for i, svc := range def.Services {
		if svc.Descriptor == nil {
			t.Fatalf("service %d missing descriptor", i)
		}
		if got := string(svc.Descriptor.FullName()); got != want[i] {
			t.Fatalf("service[%d] = %s, want %s", i, got, want[i])
		}
		methods := svc.Descriptor.Methods()
		if methods.Len() == 0 {
			t.Fatalf("%s has no methods", want[i])
		}
		methodCount += methods.Len()
	}
	if methodCount < 13 {
		t.Fatalf("method count = %d", methodCount)
	}
}

func TestNewModuleOnlyBuildsDeclaration(t *testing.T) {
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	if def.Create == nil {
		t.Fatal("Create is nil")
	}
	if def.Name != moduleName {
		t.Fatalf("name = %q", def.Name)
	}
}

func TestNewModulePassesAdminAppStaticChecksWithoutCreate(t *testing.T) {
	var created int
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	inner := def.Create
	def.Create = func(ctx context.Context, rt module.Runtime) (module.Instance, error) {
		created++
		return inner(ctx, rt)
	}
	app, err := adminapp.New(adminapp.Options{
		Config:  testConfig(t),
		Logger:  silentLogger(),
		Modules: []module.Definition{def},
	})
	if err != nil {
		t.Fatal(err)
	}
	if app == nil {
		t.Fatal("app is nil")
	}
	if created != 0 {
		t.Fatalf("Create called %d times during New", created)
	}
	if app.ListenerAddr() != "" {
		t.Fatalf("New listened on %s", app.ListenerAddr())
	}
}

func TestCreateFailsWithoutSystemTables(t *testing.T) {
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	_, err = def.Create(context.Background(), module.Runtime{
		Config: testConfig(t),
		Logger: silentLogger(),
	})
	if err == nil {
		t.Fatal("expected missing table error")
	}
	if err != ErrMissingSystemTables {
		t.Fatalf("err = %v", err)
	}
}

func testConfig(t *testing.T) adminconfig.Config {
	t.Helper()
	return adminconfig.Config{
		Env:      adminconfig.EnvDev,
		Addr:     "127.0.0.1:0",
		DBDriver: "postgres", DBDSN: "host=localhost user=test password=test dbname=test sslmode=disable",
		JWTSecret:           "test-secret-not-default",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"http://localhost:5173"},
		ShutdownTimeout:     5 * time.Second,
		DBMaxOpenConns:      5,
		DBMaxIdleConns:      2,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
		UploadDir:           t.TempDir(),
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
