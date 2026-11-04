package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestDeptServiceProtoExposesStandardSixRPCs(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_dept_proto.Services().ByName("DeptService")
	if service == nil {
		t.Fatal("DeptService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListDeptRows", "CreateDept", "GetDept", "UpdateDept", "DeleteDept", "BatchDeleteDepts",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("DeptService missing RPC %s", method)
		}
	}
}

func TestDeptStandardListIsPaged(t *testing.T) {
	listReq := adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("ListDeptRowsRequest")
	listResp := adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("ListDeptRowsResponse")
	if listReq == nil || listResp == nil {
		t.Fatal("Dept standard list request/response descriptors missing")
	}
	assertField(t, listReq, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, listReq, "keyword", 2, protoreflect.StringKind, false, false, "")
	assertField(t, listReq, "parent_id", 3, protoreflect.Int64Kind, true, false, "")
	assertField(t, listReq, "status", 4, protoreflect.EnumKind, true, false, "")
	if got := listReq.Fields().ByName("status").Enum(); got == nil || got.FullName() != adminv1.File_abysstract_admin_v1_common_proto.Enums().ByName("Status").FullName() {
		t.Fatalf("ListDeptRowsRequest.status enum = %v, want %v", got, adminv1.File_abysstract_admin_v1_common_proto.Enums().ByName("Status").FullName())
	}
	assertField(t, listReq, "orders", 5, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("OrderBy").FullName()))
	assertField(t, listResp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("Dept").FullName()))
	assertField(t, listResp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestDeptUpdateUsesOptionalPresenceWithoutChangingFieldNumbers(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("UpdateDeptRequest")
	if req == nil {
		t.Fatal("UpdateDeptRequest descriptor missing")
	}
	assertField(t, req, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, req, "parent_id", 2, protoreflect.Int64Kind, true, false, "")
	assertField(t, req, "dept_name", 3, protoreflect.StringKind, true, false, "")
	assertField(t, req, "order_num", 4, protoreflect.Int32Kind, true, false, "")
	assertField(t, req, "leader", 5, protoreflect.StringKind, true, false, "")
	assertField(t, req, "phone", 6, protoreflect.StringKind, true, false, "")
	assertField(t, req, "email", 7, protoreflect.StringKind, true, false, "")
}

func TestDeptBatchDeleteUsesInt64Keys(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("BatchDeleteDeptsRequest")
	resp := adminv1.File_abysstract_admin_v1_dept_proto.Messages().ByName("BatchDeleteDeptsResponse")
	if req == nil || resp == nil {
		t.Fatalf("Dept BatchDelete descriptors missing: request=%v response=%v", req != nil, resp != nil)
	}
	assertField(t, req, "ids", 1, protoreflect.Int64Kind, false, true, "")
}
