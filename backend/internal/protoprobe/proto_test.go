package protoprobe

import (
	"testing"

	"google.golang.org/protobuf/proto"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	testpb "github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

func TestSharedPagination(t *testing.T) {
	request := &testpb.ListWidgetsRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 20}}
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip testpb.ListWidgetsRequest
	if err := proto.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(request, &roundtrip) {
		t.Fatalf("roundtrip = %v", &roundtrip)
	}
	field := request.ProtoReflect().Descriptor().Fields().ByName("page")
	if field.Message() != (&adminv1.PageRequest{}).ProtoReflect().Descriptor() {
		t.Fatal("shared descriptor identity")
	}
	response := &testpb.ListWidgetsResponse{Page: &adminv1.PageInfo{Total: 9007199254740993}}
	if response.Page.Total != 9007199254740993 {
		t.Fatal("int64 pagination total")
	}
}
