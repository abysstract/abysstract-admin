package gen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/permission"
)

type recordingRegistrar struct {
	entries []permission.Entry
}

func (r *recordingRegistrar) RegisterGenerated(entries ...permission.Entry) error {
	r.entries = append(r.entries, entries...)
	return nil
}

func (r *recordingRegistrar) RegisterPublic(...string) error { return nil }

func TestRegisterProductPermissionsUsesPublicRegistrar(t *testing.T) {
	var registrar recordingRegistrar
	var _ permission.Registrar = &registrar
	if err := RegisterProductPermissions(&registrar); err != nil {
		t.Fatal(err)
	}
	if len(registrar.entries) == 0 {
		t.Fatal("expected generated product permissions")
	}
	found := false
	for _, entry := range registrar.entries {
		if entry.Procedure == "/company.product.v1.ProductService/ListProducts" && entry.Permission == "biz:product:list" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing list permission: %#v", registrar.entries)
	}
}
