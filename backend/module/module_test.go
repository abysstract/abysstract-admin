package module_test

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func TestDefinitionKeepsStaticFields(t *testing.T) {
	def := module.Definition{
		Name:     "billing",
		Requires: []string{"system"},
		HTTP: []module.HTTPRoute{{
			Name:   "export",
			Method: "POST",
			Path:   "/billing/export",
			Access: module.AccessAuthenticated,
		}},
		Permissions: func(permission.Registrar) error { return nil },
	}
	if def.Name != "billing" || def.HTTP[0].Access != module.AccessAuthenticated {
		t.Fatalf("definition = %#v", def)
	}
}
