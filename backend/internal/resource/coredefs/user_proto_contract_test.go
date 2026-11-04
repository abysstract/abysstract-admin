package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestUserServiceExposesReadResourceAndDedicatedSecurityWorkflows(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_user_proto.Services().ByName("UserService")
	if service == nil {
		t.Fatal("UserService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListUsers", "GetUser", "CreateUser", "UpdateUser", "DeleteUser", "BatchDeleteUsers", "ChangeUserStatus", "ResetPassword",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("UserService missing RPC %s", method)
		}
	}
}

func TestUserListWireKeepsPageFiltersAndPresence(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("ListUsersRequest")
	resp := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("ListUsersResponse")
	if req == nil || resp == nil {
		t.Fatal("User List descriptors missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")
	assertField(t, req, "status", 3, protoreflect.EnumKind, true, false, "")
	assertField(t, req, "dept_id", 4, protoreflect.Int64Kind, true, false, "")
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("User").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestUserBatchDeleteWireUsesRepeatedInt64IDs(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("BatchDeleteUsersRequest")
	resp := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("BatchDeleteUsersResponse")
	if req == nil || resp == nil {
		t.Fatal("BatchDeleteUsers descriptors missing")
	}
	assertField(t, req, "ids", 1, protoreflect.Int64Kind, false, true, "")
	assertField(t, resp, "deleted", 1, protoreflect.Int32Kind, false, false, "")
}

func TestUserUpdatePresenceDoesNotMakeRoleIDsAResourceField(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("UpdateUserRequest")
	if req == nil {
		t.Fatal("UpdateUserRequest descriptor missing")
	}
	for _, field := range []struct {
		name string
		num  protoreflect.FieldNumber
	}{
		{"id", 1}, {"nickname", 2}, {"email", 3}, {"phone", 4}, {"dept_id", 5}, {"role_ids", 6}, {"remark", 7},
	} {
		if got := req.Fields().ByName(protoreflect.Name(field.name)); got == nil || got.Number() != field.num {
			t.Fatalf("UpdateUserRequest field %s = %v, want number %d", field.name, got, field.num)
		}
	}
	for _, name := range []protoreflect.Name{"nickname", "email", "phone", "dept_id", "remark"} {
		field := req.Fields().ByName(name)
		if !field.HasOptionalKeyword() {
			t.Fatalf("UpdateUserRequest field %s lacks optional presence", name)
		}
	}
	roleIDs := req.Fields().ByName("role_ids")
	if roleIDs == nil || !roleIDs.IsList() || roleIDs.Kind() != protoreflect.Int64Kind {
		t.Fatalf("role_ids wire = %v, want repeated int64", roleIDs)
	}
}

func TestUserCreateKeepsPasswordAndRolesInDedicatedWorkflowWire(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_user_proto.Messages().ByName("CreateUserRequest")
	if req == nil {
		t.Fatal("CreateUserRequest descriptor missing")
	}
	password := req.Fields().ByName("password")
	roleIDs := req.Fields().ByName("role_ids")
	if password == nil || password.Number() != 2 || password.Kind() != protoreflect.StringKind {
		t.Fatalf("password wire = %v, want string field 2", password)
	}
	if roleIDs == nil || roleIDs.Number() != 7 || !roleIDs.IsList() || roleIDs.Kind() != protoreflect.Int64Kind {
		t.Fatalf("role_ids wire = %v, want repeated int64 field 7", roleIDs)
	}
}
