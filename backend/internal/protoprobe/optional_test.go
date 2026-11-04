package protoprobe

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	testpb "github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

func TestUpdatePresenceZeroEmptyAndOmitted(t *testing.T) {
	omitted := &testpb.UpdateWidgetRequest{Id: 7}
	if omitted.Name != nil || omitted.Enabled != nil {
		t.Fatal("omitted fields must stay unset")
	}
	data, err := proto.Marshal(omitted)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip testpb.UpdateWidgetRequest
	if err := proto.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.Name != nil || roundtrip.Enabled != nil {
		t.Fatal("omitted fields appeared after roundtrip")
	}

	empty := ""
	zero := false
	present := &testpb.UpdateWidgetRequest{Id: 7, Name: &empty, Enabled: &zero}
	if present.GetName() != "" || present.GetEnabled() {
		t.Fatal("empty/zero getters")
	}
	encoded, err := proto.Marshal(present)
	if err != nil {
		t.Fatal(err)
	}
	var decoded testpb.UpdateWidgetRequest
	if err := proto.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name == nil || decoded.Enabled == nil {
		t.Fatal("explicit empty/zero presence lost")
	}
}

func TestInt64JSONAndWire(t *testing.T) {
	const large int64 = 9007199254740993
	msg := &testpb.WidgetEntity{Id: large}
	wire, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded testpb.WidgetEntity
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Id != large {
		t.Fatalf("wire roundtrip id=%d", decoded.Id)
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("protojson lost bigint: %s", raw)
	}
}
