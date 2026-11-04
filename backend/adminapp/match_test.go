//go:build integration

package adminapp

import (
	"context"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestInstanceServiceMismatchCloses(t *testing.T) {
	skipSystemTables(t)
	svc, procedure := probeService(t, "company.contract.v1", "ProbeService", nil)
	var closes int
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{{
		Name:        "system",
		Services:    []module.ConnectService{svc},
		Permissions: publicPermissions(procedure),
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error {
				closes++
				return nil
			}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "instance has 0 services") {
		t.Fatalf("err = %v", err)
	}
	if closes != 1 {
		t.Fatalf("closes = %d", closes)
	}
}
