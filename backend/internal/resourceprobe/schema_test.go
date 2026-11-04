package resourceprobe_test

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resourceprobe/model"

	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestPublicDefinitionAndHooksCompileIndependently(t *testing.T) {
	definition := schema.Resource{Name: "ExternalProduct", Fields: []schema.Field{schema.F("Name", schema.Required(), schema.MaxLen(64))}}
	if definition.Fields[0].Name != "Name" {
		t.Fatal("public field metadata changed")
	}
	if _, err := definition.Build(); err == nil {
		t.Fatal("incomplete external definition should report validation errors")
	}
	calls := 0
	hooks := resource.ComposeHooks[model.Record](nil, &resource.HookSet[model.Record]{BeforeCreate: func(context.Context, resource.Tx, *model.Record) error { calls++; return nil }})
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, &model.Record{}); err != nil || calls != 1 {
		t.Fatalf("external hook: calls=%d err=%v", calls, err)
	}
}
