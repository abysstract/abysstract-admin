package schema

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

func roleStatusKeyResource(operation Operation) *Resource {
	resource := validUserListTarget()
	resource.Name = "RoleStatusKeyProbe"
	resource.Service = ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_role_proto.Services().ByName("RoleService"),
		Handler:    (*adminv1connect.RoleServiceHandler)(nil),
	}
	resource.Operations = []Operation{operation}
	return resource
}

func TestKeyIdentifierRequiresExactBuiltinInt64(t *testing.T) {
	operation := Operation{
		Kind: OperationDelete, Method: "ChangeRoleStatus",
		Request: (*adminv1.ChangeRoleStatusRequest)(nil), Response: (*adminv1.ChangeRoleStatusResponse)(nil),
		Binding: OperationBinding{Identifier: "Status"}, Implementation: OperationGenerated,
	}
	_, err := roleStatusKeyResource(operation).Build()
	if err == nil || !strings.Contains(err.Error(), "exact builtin int64") {
		t.Fatalf("enum Identifier must fail exact Key wire validation: %v", err)
	}
}

func TestDefaultIdentifierDoesNotCountEnumAsKeyCandidate(t *testing.T) {
	operations := StandardOperations(StandardOperationsConfig{Delete: StandardOperation{
		Method: "ChangeRoleStatus", Request: (*adminv1.ChangeRoleStatusRequest)(nil), Response: (*adminv1.ChangeRoleStatusResponse)(nil),
	}})
	if _, err := roleStatusKeyResource(operations[4]).Build(); err != nil {
		t.Fatalf("default Id must remain unambiguous when the other field is enum: %v", err)
	}
}

func TestBatchIDsRequiresExactRepeatedBuiltinInt64(t *testing.T) {
	service := ServiceConfig{
		Descriptor: testpb.File_testpb_test_proto.Services().ByName("BoolService"),
		Handler:    (*batchBoolServiceHandler)(nil),
	}
	operation := Operation{
		Kind: OperationBatchDelete, Method: "BatchDeleteBools",
		Request: (*testpb.BatchDeleteBoolsRequest)(nil), Response: (*testpb.BatchDeleteBoolsResponse)(nil),
		Binding: OperationBinding{BatchIDs: "Statuses"}, Implementation: OperationGenerated,
	}
	_, err := boolContractResource(service, []Operation{operation}).Build()
	if err == nil || !strings.Contains(err.Error(), "exact repeated builtin int64") {
		t.Fatalf("[]enum BatchIDs must fail exact Key wire validation: %v", err)
	}
}
