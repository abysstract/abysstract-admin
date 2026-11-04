package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestRoleServiceProtoExposesStandardSixAndTypedAuthorizationRPCs(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_role_proto.Services().ByName("RoleService")
	if service == nil {
		t.Fatal("RoleService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListRoles", "GetRole", "CreateRole", "UpdateRole", "DeleteRole", "BatchDeleteRoles",
		"ChangeRoleStatus", "GetRoleMenus", "UpdateRoleMenus",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("RoleService missing RPC %s", method)
		}
	}
}

func TestRoleListWireMatchesStandardPageAndStatusFilter(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("ListRolesRequest")
	resp := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("ListRolesResponse")
	if req == nil || resp == nil {
		t.Fatal("Role List descriptors missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")
	assertField(t, req, "status", 3, protoreflect.EnumKind, true, false, "")
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("Role").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestRoleUpdateUsesOptionalPresenceAndLeavesMenuIDsToCustomRPC(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("UpdateRoleRequest")
	if req == nil {
		t.Fatal("UpdateRoleRequest descriptor missing")
	}
	assertField(t, req, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, req, "role_name", 2, protoreflect.StringKind, true, false, "")
	assertField(t, req, "role_sort", 3, protoreflect.Int32Kind, true, false, "")
	assertField(t, req, "data_scope", 4, protoreflect.Int32Kind, true, false, "")
	assertField(t, req, "remark", 5, protoreflect.StringKind, true, false, "")
	if req.Fields().ByName("menu_ids") != nil {
		t.Fatal("Role Update must not expose repeated menu_ids; authorization uses custom RPC")
	}
	createReq := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("CreateRoleRequest")
	if createReq == nil || createReq.Fields().ByName("menu_ids") != nil {
		t.Fatal("Role Create must not expose repeated menu_ids; authorization uses custom RPC")
	}
}

func TestRoleBatchDeleteAndAuthorizationWireUseInt64Keys(t *testing.T) {
	batchReq := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("BatchDeleteRolesRequest")
	batchResp := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("BatchDeleteRolesResponse")
	getReq := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("GetRoleMenusRequest")
	getResp := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("GetRoleMenusResponse")
	updateReq := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("UpdateRoleMenusRequest")
	updateResp := adminv1.File_abysstract_admin_v1_role_proto.Messages().ByName("UpdateRoleMenusResponse")
	if batchReq == nil || batchResp == nil || getReq == nil || getResp == nil || updateReq == nil || updateResp == nil {
		t.Fatal("Role BatchDelete/authorization descriptors missing")
	}
	assertField(t, batchReq, "ids", 1, protoreflect.Int64Kind, false, true, "")
	assertField(t, getReq, "role_id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, getResp, "menu_ids", 1, protoreflect.Int64Kind, false, true, "")
	assertField(t, updateReq, "role_id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, updateReq, "menu_ids", 2, protoreflect.Int64Kind, false, true, "")
}
