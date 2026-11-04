package resourcegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// Formalizes tasks 12.1–12.4: the checked-in frontend schemas must be the
// current projection of the Registry plans, not hand-maintained spike output.
func TestFormalTSSchemasMatchRegistryProjection(t *testing.T) {
	plans, err := schema.BuildRegistry(coredefs.Resources())
	if err != nil {
		t.Fatalf("BuildRegistry: %v", err)
	}
	backendRoot, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	generatedDir := filepath.Join(filepath.Dir(backendRoot), "frontend", "packages", "admin-core", "src", "gen", "resource")

	for _, plan := range plans {
		name, want, err := renderTS(plan)
		if err != nil {
			t.Fatalf("renderTS(%s): %v", plan.Res.Name, err)
		}
		got, err := os.ReadFile(filepath.Join(generatedDir, name))
		if err != nil {
			t.Fatalf("read checked-in schema %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale or hand-modified; run go generate -run adminresource-gen ./internal/resource/coredefs", name)
		}
	}
}

func TestFormalTSSchemaProjectionUsesResolvedFrontendMetadata(t *testing.T) {
	plans, err := schema.BuildRegistry(coredefs.Resources())
	if err != nil {
		t.Fatalf("BuildRegistry: %v", err)
	}
	plansByName := make(map[string]*schema.Plan, len(plans))
	for _, plan := range plans {
		plansByName[plan.Res.Name] = plan
	}

	plansByName["DictData"].Fields[0].Decl.Label = "共享标签"
	labelView := buildView(plansByName["DictData"], fixtureConfig())
	if labelView.Fields[0].Label != "共享标签" {
		t.Fatalf("shared label = %q", labelView.Fields[0].Label)
	}
	_, labeledTS, err := renderTS(plansByName["DictData"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(labeledTS, []byte(`label: "共享标签"`)) {
		t.Fatal("shared label missing from generated schema")
	}

	dictData := plansByName["DictData"]
	if dictData == nil {
		t.Fatal("DictData plan missing")
	}
	view := buildView(dictData, fixtureConfig())
	if view.Name != "DictData" || view.PermPrefix != "system:dict" || view.Table != "sys_dict_data" {
		t.Fatalf("schema header = (%q, %q, %q), want DictData/system:dict/sys_dict_data", view.Name, view.PermPrefix, view.Table)
	}
	fields := make(map[string]goField, len(view.Fields))
	for _, field := range view.Fields {
		fields[field.TSName] = field
	}

	assertTSField := func(name, typ string) goField {
		t.Helper()
		field, ok := fields[name]
		if !ok {
			t.Fatalf("projected field %q missing", name)
		}
		if field.TSType != typ {
			t.Fatalf("%s tsType = %q, want %q", name, field.TSType, typ)
		}
		return field
	}

	// int64 stays bigint, ordinary integral values remain number, and time uses
	// string rather than a handwritten frontend representation.
	assertTSField("id", "bigint")
	assertTSField("dictSort", "number")
	assertTSField("createTime", "string")

	dictType := assertTSField("dictType", "string")
	if !dictType.HasRelation || dictType.RelTarget != "DictType" ||
		dictType.RelValueField != "dictType" || dictType.RelLabelField != "dictName" {
		t.Fatalf("dictType relation = %#v, want DictType/dictType/dictName", dictType)
	}
	if !dictType.InCreate || dictType.InUpdate || dictType.ServerManaged {
		t.Fatalf("dictType lifecycle = create:%v update:%v serverManaged:%v, want true/false/false", dictType.InCreate, dictType.InUpdate, dictType.ServerManaged)
	}

	remark := assertTSField("remark", "string")
	if remark.MaxLen != 500 || !remark.InCreate || !remark.InUpdate || remark.ServerManaged {
		t.Fatalf("remark metadata = %#v, want maxLen=500 and ordinary create/update lifecycle", remark)
	}
}
