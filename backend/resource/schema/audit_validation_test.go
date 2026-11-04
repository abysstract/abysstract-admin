package schema

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type resourceTestAuditEntity struct {
	ID        int64                  `protobuf:"varint,1,opt,name=id,proto3"`
	Other     string                 `protobuf:"bytes,2,opt,name=other,proto3"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,proto3"`
}

type resourceTestAuditCreate struct{}
type resourceTestAuditUpdate struct {
	ID int64 `protobuf:"varint,1,opt,name=id,proto3"`
}

type resourceTestAuditModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Other     string    `gorm:"column:other"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*resourceTestAuditModel) TableName() string { return "audit_fixture" }

type resourceTestAuditNoDBNameModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Other     string    `gorm:"column:other"`
	UpdatedAt time.Time `gorm:"-"`
}

func (*resourceTestAuditNoDBNameModel) TableName() string { return "audit_fixture_no_db_name" }

type resourceTestAuditDuplicateDBNameModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Other     string    `gorm:"column:updated_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*resourceTestAuditDuplicateDBNameModel) TableName() string {
	return "audit_fixture_duplicate_db_name"
}

type resourceTestAuditNonTimeModel struct {
	ID        int64  `gorm:"column:id;primaryKey"`
	Other     string `gorm:"column:other"`
	UpdatedAt int64  `gorm:"column:updated_at"`
}

func (*resourceTestAuditNonTimeModel) TableName() string { return "audit_fixture_non_time" }

func resourceTestAuditResource(model any) *Resource {
	return &Resource{
		Name: "ResourceAudit", PermPrefix: "test:audit-fixture", Table: "audit_fixture",
		Model: model, Entity: (*resourceTestAuditEntity)(nil),
		CreateReq: (*resourceTestAuditCreate)(nil), UpdateReq: (*resourceTestAuditUpdate)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("Other", ReadOnly()),
			F("UpdatedAt", ServerManaged()),
		},
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

func TestResourceAuditConfigNegativeDrift(t *testing.T) {
	tests := []struct {
		name   string
		model  any
		mutate func(*Resource)
		want   string
	}{
		{"missing audit declaration", (*resourceTestAuditModel)(nil), func(r *Resource) { r.Audit.UpdatedAtField = "" }, "Audit.UpdatedAtField is required"},
		{"audit field absent from model and schema", (*resourceTestAuditModel)(nil), func(r *Resource) { r.Audit.UpdatedAtField = "Missing" }, "does not exist in Model/Schema"},
		{"audit field is not time", (*resourceTestAuditNonTimeModel)(nil), nil, "exact time.Time"},
		{"audit field is not server managed", (*resourceTestAuditModel)(nil), func(r *Resource) { r.Fields[2] = F("UpdatedAt", ReadOnly()) }, "must be ServerManaged"},
		{"audit field has no GORM DBName", (*resourceTestAuditNoDBNameModel)(nil), nil, "no GORM DBName"},
		{"audit DBName conflicts with another field", (*resourceTestAuditDuplicateDBNameModel)(nil), nil, "duplicate DBName"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resourceTestAuditResource(tt.model)
			if tt.mutate != nil {
				tt.mutate(r)
			}
			if _, err := r.Build(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
