package resourcegen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

type boolViewModel struct {
	Enabled bool
}

func TestBuildViewRendersBoolDirectInEveryDirection(t *testing.T) {
	plan := boolViewPlan(t)

	view := buildView(plan, fixtureConfig())
	field := view.Fields[0]
	if field.ToEntity.Expr != "m.Enabled" || field.Create.Expr != "req.GetEnabled()" || field.Update.Expr != "req.GetEnabled()" {
		t.Fatalf("bool direct expressions changed: toEntity=%q create=%q update=%q", field.ToEntity.Expr, field.Create.Expr, field.Update.Expr)
	}
	if field.ToEntity.IsEnum || field.Create.IsEnum || field.Update.IsEnum || field.Create.NeedsBounds || field.Update.NeedsBounds {
		t.Fatalf("bool direct conversion must not use enum/bounds paths: %+v", field)
	}
}

func boolViewPlan(t *testing.T) *schema.Plan {
	t.Helper()
	boolType := reflect.TypeOf(false)
	return &schema.Plan{
		Res:        &schema.Resource{Name: "Bool", PermPrefix: "test:bool", Table: "bools"},
		ModelType:  reflect.TypeOf(boolViewModel{}),
		EntityType: reflect.TypeOf(testpb.BoolEntity{}),
		CreateType: reflect.TypeOf(testpb.CreateBoolRequest{}),
		UpdateType: reflect.TypeOf(testpb.UpdateBoolRequest{}),
		Fields: []*schema.FieldPlan{{
			ModelField: "Enabled", ModelType: boolType, ProtoName: "enabled", ProtoGoName: "Enabled", JSONName: "enabled", DBName: "enabled",
			EntityType: boolType, InCreate: true, InUpdate: true, UpdateOptional: true,
			ModelToEntity: schema.ConversionPlan{Kind: schema.ConvDirect, SourceType: boolType, TargetType: boolType},
			CreateToModel: schema.ConversionPlan{Kind: schema.ConvDirect, SourceType: boolType, TargetType: boolType},
			UpdateToModel: schema.ConversionPlan{Kind: schema.ConvDirect, SourceType: boolType, TargetType: boolType},
			Decl:          schema.F("Enabled", schema.Filterable()),
		}},
	}
}

func TestNullableBoolConverterPreservesAbsentCreateAndFalseUpdate(t *testing.T) {
	plan := boolViewPlan(t)
	field := plan.Fields[0]
	scalar := reflect.TypeOf(false)
	pointer := reflect.PointerTo(scalar)
	field.ModelType, field.EntityType = pointer, pointer
	field.CreateOptional = true
	field.ModelToEntity = schema.ConversionPlan{Kind: schema.ConvDirect, SourceType: pointer, TargetType: pointer}
	field.CreateToModel = schema.ConversionPlan{Kind: schema.ConvDirect, SourceType: scalar, TargetType: pointer}
	field.UpdateToModel = field.CreateToModel
	view := buildView(plan, fixtureConfig())
	if !view.Fields[0].ModelNullable || !view.Fields[0].CreateOptional {
		t.Fatal("nullable storage/presence missing from generation plan")
	}
	source := string(renderGoConvSource(t, plan))
	for _, expected := range []string{"e.Enabled = m.Enabled", "if req.Enabled != nil", "new(req.GetEnabled())", "*nv != *m.Enabled"} {
		if !strings.Contains(source, expected) {
			t.Fatalf("nullable conversion missing %q: %s", expected, source)
		}
	}
}
