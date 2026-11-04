package adminapp

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func TestNewPermissionCoverageAndOwnership(t *testing.T) {
	cfg := testConfig(t)
	svc, procedure := probeService(t, "company.contract.v1", "ProbeService", nil)
	other, otherProc := probeService(t, "company.contract.v1", "OtherService", nil)

	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{{
		Name:     "system",
		Services: []module.ConnectService{svc},
		Create:   emptyCreate,
	}}})
	if err == nil || !strings.Contains(err.Error(), "missing declaration") {
		t.Fatalf("missing permission err = %v", err)
	}

	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{{
		Name:        "system",
		Services:    []module.ConnectService{svc},
		Permissions: publicPermissions("/company.contract.v1.ProbeService/Pong"),
		Create:      emptyCreate,
	}}})
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("misspelled err = %v", err)
	}

	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{
			Name:        "system",
			Services:    []module.ConnectService{svc},
			Permissions: publicPermissions(procedure),
			Create:      emptyCreate,
		},
		{
			Name:        "billing",
			Requires:    []string{"system"},
			Services:    []module.ConnectService{other},
			Permissions: publicPermissions(procedure),
			Create:      emptyCreate,
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("cross-module err = %v", err)
	}

	app, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{{
		Name:     "system",
		Services: []module.ConnectService{svc},
		Permissions: func(reg permission.Registrar) error {
			if err := reg.RegisterGenerated(permission.Entry{Procedure: procedure, Permission: permission.Authenticated}); err != nil {
				return err
			}
			return reg.RegisterGenerated(permission.Entry{Procedure: procedure, Permission: permission.Authenticated})
		},
		Create: emptyCreate,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.registry.RegisterPublic(otherProc); err == nil {
		t.Fatal("expected frozen registrar to reject writes")
	}
}
