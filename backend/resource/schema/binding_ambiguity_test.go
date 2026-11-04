package schema

import (
	"context"
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

func standardUserUpdate(identifier string) Operation {
	operations := StandardOperations(StandardOperationsConfig{
		Update: StandardOperation{
			Method: "UpdateUser", Request: (*adminv1.UpdateUserRequest)(nil), Response: (*adminv1.UpdateUserResponse)(nil),
			Binding: OperationBinding{Identifier: identifier, Item: "User"},
		},
	})
	return operations[3]
}

func TestStandardUpdateRequiresExplicitIdentifierWhenRequestHasMultipleKeyCandidates(t *testing.T) {
	resource := validUserListTarget()
	resource.Operations = append(resource.Operations, standardUserUpdate(""))
	_, err := resource.Build()
	if err == nil || !strings.Contains(err.Error(), "explicit Identifier") {
		t.Fatalf("default Identifier with id+dept_id candidates must fail: %v", err)
	}

	resource = validUserListTarget()
	resource.Operations = append(resource.Operations, standardUserUpdate("Id"))
	if _, err := resource.Build(); err != nil {
		t.Fatalf("explicit Identifier must resolve ambiguity: %v", err)
	}
}

type batchBoolServiceHandler interface {
	BatchDeleteBools(context.Context, *testpb.BatchDeleteBoolsRequest) (*testpb.BatchDeleteBoolsResponse, error)
}

func standardBoolBatchDelete(batchIDs string) Operation {
	operations := StandardOperations(StandardOperationsConfig{
		BatchDelete: StandardOperation{
			Method: "BatchDeleteBools", Request: (*testpb.BatchDeleteBoolsRequest)(nil), Response: (*testpb.BatchDeleteBoolsResponse)(nil),
			Binding: OperationBinding{BatchIDs: batchIDs},
		},
	})
	return operations[5]
}

func TestStandardBatchDeleteRequiresExplicitBindingWhenRequestHasMultipleRepeatedIDs(t *testing.T) {
	service := ServiceConfig{
		Descriptor: testpb.File_testpb_test_proto.Services().ByName("BoolService"),
		Handler:    (*batchBoolServiceHandler)(nil),
	}
	resource := boolContractResource(service, []Operation{standardBoolBatchDelete("")})
	_, err := resource.Build()
	if err == nil || !strings.Contains(err.Error(), "explicit BatchIDs") {
		t.Fatalf("default BatchIDs with two repeated int64 candidates must fail: %v", err)
	}

	resource = boolContractResource(service, []Operation{standardBoolBatchDelete("Ids")})
	if _, err := resource.Build(); err != nil {
		t.Fatalf("explicit BatchIDs must resolve ambiguity: %v", err)
	}
}
