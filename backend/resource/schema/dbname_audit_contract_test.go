package schema

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type invalidColumnsModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	A         string    `gorm:"column:dup"`
	B         string    `gorm:"column:dup"`
	Ignored   string    `gorm:"-"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*invalidColumnsModel) TableName() string { return "invalid_columns" }

type invalidColumnsEntity struct {
	ID        int64                  `protobuf:"varint,1,opt,name=id,proto3"`
	A         string                 `protobuf:"bytes,2,opt,name=a,proto3"`
	B         string                 `protobuf:"bytes,3,opt,name=b,proto3"`
	Ignored   string                 `protobuf:"bytes,4,opt,name=ignored,proto3"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=updated_at,json=updatedAt,proto3"`
}

type invalidColumnsCreate struct {
	A       string `protobuf:"bytes,1,opt,name=a,proto3"`
	B       string `protobuf:"bytes,2,opt,name=b,proto3"`
	Ignored string `protobuf:"bytes,3,opt,name=ignored,proto3"`
}

type invalidColumnsUpdate struct {
	ID      int64   `protobuf:"varint,1,opt,name=id,proto3"`
	A       *string `protobuf:"bytes,2,opt,name=a,proto3,oneof"`
	B       *string `protobuf:"bytes,3,opt,name=b,proto3,oneof"`
	Ignored *string `protobuf:"bytes,4,opt,name=ignored,proto3,oneof"`
}

func invalidColumnsResource(fields []Field) *Resource {
	return &Resource{
		Name: "InvalidColumns", PermPrefix: "test:columns", Table: "invalid_columns",
		Model: (*invalidColumnsModel)(nil), Entity: (*invalidColumnsEntity)(nil),
		CreateReq: (*invalidColumnsCreate)(nil), UpdateReq: (*invalidColumnsUpdate)(nil),
		Fields: fields,
		Key:    KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

func TestBuildRejectsMissingAndDuplicateGORMDBNames(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		r := invalidColumnsResource([]Field{
			F("ID", ServerManaged()), F("A"), F("B"), F("Ignored"), F("UpdatedAt", ServerManaged()),
		})
		_, err := r.Build()
		if err == nil || !strings.Contains(err.Error(), "duplicate DBName") {
			t.Fatalf("duplicate DBName must fail: %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		r := invalidColumnsResource([]Field{
			F("ID", ServerManaged()), F("A"), F("B", ReadOnly()), F("Ignored"), F("UpdatedAt", ServerManaged()),
		})
		r.EntityIgnore = []string{"b"}
		_, err := r.Build()
		if err == nil || !strings.Contains(err.Error(), "no GORM DBName") {
			t.Fatalf("missing DBName must fail: %v", err)
		}
	})
}

type badAuditModel struct {
	ID        int64 `gorm:"column:id;primaryKey"`
	UpdatedAt int64 `gorm:"column:updated_at"`
}

func (*badAuditModel) TableName() string { return "bad_audit" }

type badAuditEntity struct {
	ID        int64 `protobuf:"varint,1,opt,name=id,proto3"`
	UpdatedAt int64 `protobuf:"varint,2,opt,name=updated_at,json=updatedAt,proto3"`
}

type badAuditCreate struct{}
type badAuditUpdate struct {
	ID int64 `protobuf:"varint,1,opt,name=id,proto3"`
}

func TestBuildRejectsAuditFieldThatIsNotExactTime(t *testing.T) {
	r := &Resource{
		Name: "BadAudit", PermPrefix: "test:audit", Table: "bad_audit",
		Model: (*badAuditModel)(nil), Entity: (*badAuditEntity)(nil), CreateReq: (*badAuditCreate)(nil), UpdateReq: (*badAuditUpdate)(nil),
		Fields: []Field{F("ID", ServerManaged()), F("UpdatedAt", ServerManaged())},
		Key:    KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "exact time.Time") {
		t.Fatalf("non-time audit field must fail: %v", err)
	}
}

type managedInputModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Secret    string    `gorm:"column:secret"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*managedInputModel) TableName() string { return "managed_input" }

type managedInputEntity struct {
	ID        int64                  `protobuf:"varint,1,opt,name=id,proto3"`
	Secret    string                 `protobuf:"bytes,2,opt,name=secret,proto3"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,proto3"`
}

type managedInputCreate struct{}
type managedInputUpdate struct {
	ID     int64   `protobuf:"varint,1,opt,name=id,proto3"`
	Secret *string `protobuf:"bytes,2,opt,name=secret,proto3,oneof"`
}

func TestBuildAllowsOnlyConfiguredKeyAsServerManagedIdentifier(t *testing.T) {
	r := &Resource{
		Name: "ManagedInput", PermPrefix: "test:managed", Table: "managed_input",
		Model: (*managedInputModel)(nil), Entity: (*managedInputEntity)(nil), CreateReq: (*managedInputCreate)(nil), UpdateReq: (*managedInputUpdate)(nil),
		Fields: []Field{F("ID", ServerManaged()), F("Secret", ServerManaged()), F("UpdatedAt", ServerManaged())},
		Key:    KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "only Key") {
		t.Fatalf("non-key ServerManaged Update field must fail: %v", err)
	}
}
