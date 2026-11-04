package permission_test

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/permission"
)

type recordingRegistrar struct {
	generated []permission.Entry
	public    []string
}

func (r *recordingRegistrar) RegisterGenerated(entries ...permission.Entry) error {
	r.generated = append(r.generated, entries...)
	return nil
}

func (r *recordingRegistrar) RegisterPublic(procedures ...string) error {
	r.public = append(r.public, procedures...)
	return nil
}

func TestRegistrarRecordsGeneratedAndPublicEntries(t *testing.T) {
	var registrar recordingRegistrar
	var _ permission.Registrar = &registrar

	if err := registrar.RegisterGenerated(permission.Entry{
		Procedure:  "/company.contract.v1.ContractService/ListContracts",
		Permission: "biz:contract:list",
	}); err != nil {
		t.Fatal(err)
	}
	if err := registrar.RegisterPublic("/company.contract.v1.ContractService/Health"); err != nil {
		t.Fatal(err)
	}
	if len(registrar.generated) != 1 || registrar.generated[0].Permission != "biz:contract:list" {
		t.Fatalf("generated = %#v", registrar.generated)
	}
	if len(registrar.public) != 1 || registrar.public[0] != "/company.contract.v1.ContractService/Health" {
		t.Fatalf("public = %#v", registrar.public)
	}
}
