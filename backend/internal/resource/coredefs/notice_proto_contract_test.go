package coredefs

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestNoticeServiceProtoExposesStandardSixRPCs(t *testing.T) {
	service := adminv1.File_abysstract_admin_v1_notice_proto.Services().ByName("NoticeService")
	if service == nil {
		t.Fatal("NoticeService descriptor missing")
	}
	for _, method := range []protoreflect.Name{
		"ListNotices", "CreateNotice", "GetNotice", "UpdateNotice", "DeleteNotice", "BatchDeleteNotices",
	} {
		if service.Methods().ByName(method) == nil {
			t.Fatalf("NoticeService missing RPC %s", method)
		}
	}
}

func TestNoticeListWireMatchesStandardPageContract(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("ListNoticesRequest")
	if req == nil {
		t.Fatal("ListNoticesRequest descriptor missing")
	}
	assertField(t, req, "page", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageRequest").FullName()))
	assertField(t, req, "keyword", 2, protoreflect.StringKind, false, false, "")

	resp := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("ListNoticesResponse")
	if resp == nil {
		t.Fatal("ListNoticesResponse descriptor missing")
	}
	assertField(t, resp, "items", 1, protoreflect.MessageKind, false, true, string(adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("Notice").FullName()))
	assertField(t, resp, "page", 2, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_common_proto.Messages().ByName("PageInfo").FullName()))
}

func TestNoticeUpdateWireUsesOptionalPresenceWithoutChangingFieldNumbers(t *testing.T) {
	req := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("UpdateNoticeRequest")
	if req == nil {
		t.Fatal("UpdateNoticeRequest descriptor missing")
	}
	assertField(t, req, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, req, "notice_title", 2, protoreflect.StringKind, true, false, "")
	assertField(t, req, "notice_type", 3, protoreflect.Int32Kind, true, false, "")
	assertField(t, req, "notice_content", 4, protoreflect.StringKind, true, false, "")
	assertField(t, req, "remark", 5, protoreflect.StringKind, true, false, "")
}

func TestNoticeGetAndBatchDeleteWireUsesInt64Keys(t *testing.T) {
	getReq := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("GetNoticeRequest")
	getResp := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("GetNoticeResponse")
	batchReq := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("BatchDeleteNoticesRequest")
	batchResp := adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("BatchDeleteNoticesResponse")
	if getReq == nil || getResp == nil || batchReq == nil || batchResp == nil {
		t.Fatalf("Notice get/batch descriptors missing: getReq=%v getResp=%v batchReq=%v batchResp=%v", getReq != nil, getResp != nil, batchReq != nil, batchResp != nil)
	}
	assertField(t, getReq, "id", 1, protoreflect.Int64Kind, false, false, "")
	assertField(t, getResp, "item", 1, protoreflect.MessageKind, false, false, string(adminv1.File_abysstract_admin_v1_notice_proto.Messages().ByName("Notice").FullName()))
	assertField(t, batchReq, "ids", 1, protoreflect.Int64Kind, false, true, "")
}
