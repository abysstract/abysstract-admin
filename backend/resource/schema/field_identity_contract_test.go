package schema

import (
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

type duplicateProtoModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	EnabledA  bool      `gorm:"column:enabled_a"`
	EnabledB  bool      `gorm:"column:enabled_b"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*duplicateProtoModel) TableName() string { return "duplicate_proto" }

func TestBuildRejectsMultipleSchemaFieldsMappedToSameEntityField(t *testing.T) {
	resource := &Resource{
		Name: "DuplicateProto", PermPrefix: "test:duplicate-proto", Table: "duplicate_proto",
		Model: (*duplicateProtoModel)(nil), Entity: (*testpb.BoolEntity)(nil),
		CreateReq: (*testpb.CreateBoolRequest)(nil), UpdateReq: (*testpb.UpdateBoolRequest)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("EnabledA", ProtoName("enabled")),
			F("EnabledB", ProtoName("enabled")),
			F("UpdatedAt", ServerManaged()),
		},
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}

	_, err := resource.Build()
	if err == nil {
		t.Fatal("duplicate resolved Entity field mapping must fail Build")
	}
	for _, want := range []string{"ProtoName", "ProtoGoName", "JSONName"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Build error must report duplicate %s: %v", want, err)
		}
	}
}
