package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestMenuServiceProtoExposesStandardSixRPCs(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_menu_proto.Services().ByName("MenuService")
	if service == nil {
		t.Fatal("MenuService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListMenus", "GetMenu", "CreateMenu", "UpdateMenu", "DeleteMenu", "BatchDeleteMenus",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("MenuService missing RPC %s", method)
		}
	}
}

func TestMenuListWireMatchesStandardFlatList(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("ListMenusRequest")
	resp := adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("ListMenusResponse")
	if req == nil || resp == nil {
		t.Fatal("Menu List descriptors missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")
	assertField(t, req, "parent_id", 3, protoreflect.Int64Kind, true, false, "")
	assertField(t, req, "menu_type", 4, protoreflect.StringKind, true, false, "")
	assertField(t, req, "status", 5, protoreflect.EnumKind, true, false, "")
	assertField(t, req, "orders", 6, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("OrderBy").FullName()))
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("Menu").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestMenuUpdateUsesOptionalPresenceWithoutChangingFieldNumbers(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("UpdateMenuRequest")
	if req == nil {
		t.Fatal("UpdateMenuRequest descriptor missing")
	}
	assertField(t, req, "id", 1, protoreflect.Int64Kind, false, false, "")
	for _, field := range []struct {
		name   protoreflect.Name
		number protoreflect.FieldNumber
		kind   protoreflect.Kind
	}{
		{"menu_name", 2, protoreflect.StringKind},
		{"parent_id", 3, protoreflect.Int64Kind},
		{"order_num", 4, protoreflect.Int32Kind},
		{"path", 5, protoreflect.StringKind},
		{"component", 6, protoreflect.StringKind},
		{"query", 7, protoreflect.StringKind},
		{"is_frame", 8, protoreflect.Int32Kind},
		{"is_cache", 9, protoreflect.Int32Kind},
		{"menu_type", 10, protoreflect.StringKind},
		{"visible", 11, protoreflect.Int32Kind},
		{"perms", 12, protoreflect.StringKind},
		{"icon", 13, protoreflect.StringKind},
	} {
		assertField(t, req, field.name, field.number, field.kind, true, false, "")
	}
}

func TestMenuBatchDeleteUsesInt64Keys(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("BatchDeleteMenusRequest")
	resp := adminv1.File_abysstract_admin_v1_menu_proto.Messages().ByName("BatchDeleteMenusResponse")
	if req == nil || resp == nil {
		t.Fatalf("Menu BatchDelete descriptors missing: request=%v response=%v", req != nil, resp != nil)
	}
	assertField(t, req, "ids", 1, protoreflect.Int64Kind, false, true, "")
}
