package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestConfigServiceProtoExposesStandardSixRPCs(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_config_proto.Services().ByName("ConfigService")
	if service == nil {
		t.Fatal("ConfigService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListConfigs", "CreateConfig", "GetConfig", "UpdateConfig", "DeleteConfig", "BatchDeleteConfigs",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("ConfigService missing RPC %s", method)
		}
	}
}

func TestConfigListWireMatchesDeclaredReachability(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("ListConfigsRequest")
	if req == nil {
		t.Fatal("ListConfigsRequest descriptor missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")

	resp := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("ListConfigsResponse")
	if resp == nil {
		t.Fatal("ListConfigsResponse descriptor missing")
	}
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("Config").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestConfigUpdateWireUsesOptionalPresence(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("UpdateConfigRequest")
	if req == nil {
		t.Fatal("UpdateConfigRequest descriptor missing")
	}
	assertField(t, req, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, req, "config_name", 2, protoreflect.StringKind, true, false, "")
	assertField(t, req, "config_value", 3, protoreflect.StringKind, true, false, "")
	assertField(t, req, "remark", 4, protoreflect.StringKind, true, false, "")
}

func TestConfigGetAndBatchDeleteWire(t *testing.T) {
	getReq := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("GetConfigRequest")
	getResp := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("GetConfigResponse")
	batchReq := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("BatchDeleteConfigsRequest")
	batchResp := adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("BatchDeleteConfigsResponse")
	if getReq == nil || getResp == nil || batchReq == nil || batchResp == nil {
		t.Fatalf("Config get/batch message descriptors missing: getReq=%v getResp=%v batchReq=%v batchResp=%v", getReq != nil, getResp != nil, batchReq != nil, batchResp != nil)
	}
	assertField(t, getReq, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, getResp, "item", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_config_proto.Messages().ByName("Config").FullName()))
	assertField(t, batchReq, "ids", 1, protoreflect.Int64Kind, false, true, "")
}
