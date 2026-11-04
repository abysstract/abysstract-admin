package coredefs

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestConfigResourceDeclaresGeneratedInjectedStandardCRUD(t *testing.T) {
	var config *schema.Resource
	for _, resource := range Resources() {
		if resource.Name == "Config" {
			config = resource
			break
		}
	}
	if config == nil {
		t.Fatal("Config resource is not registered")
	}
	if config.Mode != schema.Generated {
		t.Fatalf("Config mode = %v, want Generated", config.Mode)
	}
	if config.Repository.Implementation != schema.RepositoryInjected {
		t.Fatalf("Config repository implementation = %v, want RepositoryInjected", config.Repository.Implementation)
	}
	if config.Audit.UpdatedAtField != "UpdateTime" {
		t.Fatalf("Config UpdatedAtField = %q, want UpdateTime", config.Audit.UpdatedAtField)
	}
	if len(config.Operations) != 6 {
		t.Fatalf("Config operation count = %d, want standard six", len(config.Operations))
	}
	for _, operation := range config.Operations {
		if operation.Implementation != schema.OperationGenerated {
			t.Fatalf("Config operation %s implementation = %v, want OperationGenerated", operation.Method, operation.Implementation)
		}
	}

	fields := make(map[string]schema.Field, len(config.Fields))
	for _, field := range config.Fields {
		fields[field.Name] = field
	}
	if !fields["ConfigName"].Searchable || !fields["ConfigKey"].Searchable {
		t.Fatal("Config keyword search must preserve both config_name and config_key behavior")
	}
	for _, name := range []string{"ConfigName", "ConfigKey", "ConfigValue", "ConfigType", "Remark"} {
		field, ok := fields[name]
		if !ok {
			t.Fatalf("Config field %s missing", name)
		}
		if field.Filterable || field.Sortable {
			t.Fatalf("Config field %s declares unreachable filter/sort capability", name)
		}
	}
	if fields["ConfigKey"].Lifecycle.EffUpdatable() || fields["ConfigType"].Lifecycle.EffUpdatable() {
		t.Fatal("Config create-only fields must not be updatable")
	}
}
