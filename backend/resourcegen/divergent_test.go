package resourcegen

import (
	"go/format"
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func divergentResource(t *testing.T) *schema.Resource {
	t.Helper()
	return &schema.Resource{
		Name: "DictData", PermPrefix: "system:dict", Mode: schema.Generated, Table: model.TableNameSysDictDatum,
		Model: (*model.SysDictDatum)(nil), Entity: (*adminv1.DictData)(nil),
		CreateReq: (*adminv1.CreateDictDataRequest)(nil), UpdateReq: (*adminv1.UpdateDictDataRequest)(nil),
		Fields: []schema.Field{
			schema.F("ID", schema.ServerManaged()),
			schema.F("DictSort"),
			schema.F("DictLabel", schema.Required(), schema.MaxLen(100)),
			schema.F("DictValue", schema.Required(), schema.MaxLen(100)),
			schema.F("DictType", schema.Required(), schema.MaxLen(100), schema.CreateOnly()),
			schema.F("CSSClass", schema.ReadOnly()),
			schema.F("ListClass", schema.ReadOnly()),
			schema.F("IsDefault"),
			schema.F("Status", schema.EnumMap(map[string]int16{"STATUS_ENABLED": 1, "STATUS_DISABLED": 2}), schema.ReadOnly()),
			schema.F("Remark", schema.MaxLen(500)),
			schema.F("CreateTime", schema.ServerManaged()),
			schema.F("UpdateTime", schema.ServerManaged()),
		},
		Key: schema.KeyConfig{ModelField: "ID"}, Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
	}
}

func divergentResourcePlan(t *testing.T) *schema.Plan {
	t.Helper()
	plan, err := divergentResource(t).Build()
	if err != nil {
		t.Fatalf("build divergent fixture: %v", err)
	}
	return plan
}

// End-to-end Build -> renderGo -> go/format verifies that independently
// derived direction plans drive divergent enum/narrow conversions.
func TestDirectionIndependent_EndToEnd(t *testing.T) {
	p := divergentResourcePlan(t)

	var statusOK bool
	for _, f := range p.Fields {
		if f.ProtoName == "status" {
			statusOK = true
			if f.ModelToEntity.Kind != schema.ConvEnum {
				t.Fatalf("status ModelToEntity = %v, want ConvEnum", f.ModelToEntity.Kind)
			}
		}
	}
	if !statusOK {
		t.Fatal("status field missing")
	}

	src := renderGoConvSource(t, p)
	if _, err := format.Source(src); err != nil {
		t.Fatalf("generated source is not gofmt-compatible: %v", err)
	}
	code := string(src)

	for _, f := range p.Fields {
		if f.ProtoName == "is_default" && (f.CreateToModel.Kind != schema.ConvNarrow || f.UpdateToModel.Kind != schema.ConvNarrow) {
			t.Fatalf("is_default plans: create=%v update=%v, want ConvNarrow", f.CreateToModel.Kind, f.UpdateToModel.Kind)
		}
	}
	if !strings.Contains(code, "int16(req.GetIsDefault())") {
		t.Fatal("is_default must render bounded int16 conversion, not enum conversion")
	}
}
