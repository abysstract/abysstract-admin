package adminapp

import (
	"errors"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestNewRequiresSystemModule(t *testing.T) {
	var created atomic.Int32
	_, err := New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: nil})
	if !errors.Is(err, ErrMissingSystemModule) {
		t.Fatalf("empty modules err = %v", err)
	}
	_, err = New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{{
		Name:   "billing",
		Create: countingCreate(&created),
	}}})
	if !errors.Is(err, ErrMissingSystemModule) {
		t.Fatalf("missing system err = %v", err)
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d times", created.Load())
	}
}

func TestNewRejectsInvalidAndDuplicateNames(t *testing.T) {
	var created atomic.Int32
	cfg := testConfig(t)
	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "System", Create: countingCreate(&created)},
	}})
	if err == nil {
		t.Fatal("expected invalid name")
	}
	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "system", Create: countingCreate(&created)},
		{Name: "system", Create: countingCreate(&created)},
	}})
	if err == nil {
		t.Fatal("expected duplicate")
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d times", created.Load())
	}
}

func TestNewRejectsMissingSelfAndCyclicDependencies(t *testing.T) {
	var created atomic.Int32
	cfg := testConfig(t)
	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "system", Create: countingCreate(&created)},
		{Name: "billing", Requires: []string{"missing"}, Create: countingCreate(&created)},
	}})
	if err == nil {
		t.Fatal("expected missing dependency")
	}
	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "system", Requires: []string{"system"}, Create: countingCreate(&created)},
	}})
	if err == nil {
		t.Fatal("expected self dependency")
	}
	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "system", Requires: []string{"a"}, Create: countingCreate(&created)},
		{Name: "a", Requires: []string{"b"}, Create: countingCreate(&created)},
		{Name: "b", Requires: []string{"system"}, Create: countingCreate(&created)},
	}})
	if err == nil {
		t.Fatal("expected cycle")
	}
	if created.Load() != 0 {
		t.Fatalf("Create called %d times", created.Load())
	}
}

func TestNewSortsIndependentsAndLayersStably(t *testing.T) {
	cfg := testConfig(t)
	first, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{Name: "zeta", Requires: []string{"system"}, Create: emptyCreate},
		{Name: "alpha", Requires: []string{"system"}, Create: emptyCreate},
		systemModule(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		systemModule(),
		{Name: "alpha", Requires: []string{"system"}, Create: emptyCreate},
		{Name: "zeta", Requires: []string{"system"}, Create: emptyCreate},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(first.modules), names(second.modules); got != want {
		t.Fatalf("order %q vs %q", got, want)
	}
	if names(first.modules) != "system,alpha,zeta" {
		t.Fatalf("order = %s", names(first.modules))
	}
}

func names(defs []module.Definition) string {
	out := ""
	for i, def := range defs {
		if i > 0 {
			out += ","
		}
		out += def.Name
	}
	return out
}

func TestNewCopiesNestedSlices(t *testing.T) {
	cfg := testConfig(t)
	requires := []string{"system"}
	httpRoutes := []module.HTTPRoute{{
		Name:   "export",
		Method: "POST",
		Path:   "/billing/export",
		Access: module.AccessAuthenticated,
	}}
	app, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		systemModule(),
		{Name: "billing", Requires: requires, HTTP: httpRoutes, Create: emptyCreate},
	}})
	if err != nil {
		t.Fatal(err)
	}
	requires[0] = "mutated"
	httpRoutes[0].Path = "/mutated"
	if app.modules[1].Requires[0] != "system" {
		t.Fatalf("requires mutated: %v", app.modules[1].Requires)
	}
	if app.modules[1].HTTP[0].Path != "/billing/export" {
		t.Fatalf("http mutated: %v", app.modules[1].HTTP)
	}
}

func TestNewRejectsStreamingMethods(t *testing.T) {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("company/contract/v1/stream.proto"),
		Package:    proto.String("company.contract.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/wrappers.proto"},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("StreamService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:            proto.String("Watch"),
				InputType:       proto.String(".google.protobuf.StringValue"),
				OutputType:      proto.String(".google.protobuf.StringValue"),
				ServerStreaming: proto.Bool(true),
			}},
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	svc := module.ConnectService{Descriptor: file.Services().ByName("StreamService"), Register: func(*connect.Server) {}}
	_, err = New(Options{Config: testConfig(t), Logger: silentLogger(), Modules: []module.Definition{{
		Name:     "system",
		Services: []module.ConnectService{svc},
		Create:   emptyCreate,
	}}})
	if err == nil {
		t.Fatal("expected streaming error")
	}
}
