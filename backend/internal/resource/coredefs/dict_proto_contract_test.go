package coredefs

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestDictServiceProtoExposesStandardSixRPCsForTypeAndData(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_dict_proto.Services().ByName("DictService")
	if service == nil {
		t.Fatal("DictService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListDictTypes", "CreateDictType", "GetDictType", "UpdateDictType", "DeleteDictType", "BatchDeleteDictTypes",
		"ListDictData", "CreateDictData", "GetDictData", "UpdateDictData", "DeleteDictData", "BatchDeleteDictData",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("DictService missing RPC %s", method)
		}
	}
}

func TestDictTypeListWireMatchesStandardListBinding(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("ListDictTypesRequest")
	if req == nil {
		t.Fatal("ListDictTypesRequest descriptor missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")
	assertField(t, req, "dict_type", 3, protoreflect.StringKind, true, false, "")
	assertField(t, req, "orders", 4, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("OrderBy").FullName()))

	resp := adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("ListDictTypesResponse")
	if resp == nil {
		t.Fatal("ListDictTypesResponse descriptor missing")
	}
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("DictType").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestDictDataListWireMatchesStandardListBinding(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("ListDictDataRequest")
	if req == nil {
		t.Fatal("ListDictDataRequest descriptor missing")
	}
	assertField(t, req, "dict_type", 1, protoreflect.StringKind, true, false, "")
	assertField(t, req, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 3, protoreflect.StringKind, false, false, "")
	assertField(t, req, "orders", 4, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("OrderBy").FullName()))

	resp := adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("ListDictDataResponse")
	if resp == nil {
		t.Fatal("ListDictDataResponse descriptor missing")
	}
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_dict_proto.Messages().ByName("DictData").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func assertField(t *testing.T, message protoreflect.MessageDescriptor, name protoreflect.Name, number protoreflect.FieldNumber, kind protoreflect.Kind, optional bool, repeated bool, messageFullName string) {
	t.Helper()
	field := message.Fields().ByName(name)
	if field == nil {
		t.Fatalf("%s missing field %s", message.FullName(), name)
	}
	if field.Number() != number {
		t.Fatalf("%s.%s number = %d, want %d", message.FullName(), name, field.Number(), number)
	}
	if field.Kind() != kind {
		t.Fatalf("%s.%s kind = %v, want %v", message.FullName(), name, field.Kind(), kind)
	}
	if field.IsList() != repeated {
		t.Fatalf("%s.%s repeated = %v, want %v", message.FullName(), name, field.IsList(), repeated)
	}
	if field.HasPresence() != optional && kind != protoreflect.MessageKind {
		t.Fatalf("%s.%s presence = %v, want %v", message.FullName(), name, field.HasPresence(), optional)
	}
	if messageFullName != "" {
		if field.Message() == nil || string(field.Message().FullName()) != messageFullName {
			t.Fatalf("%s.%s message = %v, want %s", message.FullName(), name, field.Message().FullName(), messageFullName)
		}
	}
	if optional {
		// protobuf optional scalar fields synthesize presence in descriptors; a
		// nil oneof marks an old non-optional scalar and must be rejected here.
		if field.ContainingOneof() == nil || !field.ContainingOneof().IsSynthetic() {
			// Message fields also have presence but are not represented as optional
			// scalars, so this check applies only to scalar optionals.
			if kind != protoreflect.MessageKind {
				t.Fatalf("%s.%s is not proto3 optional", message.FullName(), name)
			}
		}
	}
}

func TestDictProtoGeneratedMessagesRemainReflectable(t *testing.T) {
	for _, msg := range []proto.Message{
		&adminv1.ListDictTypesRequest{},
		&adminv1.ListDictDataRequest{},
	} {
		if msg.ProtoReflect().Descriptor() == nil {
			t.Fatalf("%T missing descriptor", msg)
		}
	}
}
