package system

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	"github.com/abysstract/abysstract-admin/backend/module"
)

func waitListening(t *testing.T, app *adminapp.App, runErr <-chan error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if app.ListenerAddr() != "" {
			return
		}
		select {
		case err := <-runErr:
			t.Fatalf("run ended before listen: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not listen")
}

func probeService(t *testing.T, pkg, name string, onCall func()) (module.ConnectService, string) {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("company/contract/v1/" + name + ".proto"),
		Package:    proto.String(pkg),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/wrappers.proto"},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String(name),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("Ping"),
				InputType:  proto.String(".google.protobuf.StringValue"),
				OutputType: proto.String(".google.protobuf.StringValue"),
			}},
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	service := file.Services().ByName(protoreflect.Name(name))
	procedure := "/" + string(service.FullName()) + "/Ping"
	return module.ConnectService{
		Descriptor: service,
		Register: func(server *connect.Server) {
			server.Register(connect.Method{Spec: connect.Spec{Procedure: procedure, Schema: service.Methods().ByName("Ping")}, Handler: func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				var req wrapperspb.StringValue
				if err := stream.Receive(&req); err != nil {
					return err
				}
				if onCall != nil {
					onCall()
				}
				return stream.Send(&wrapperspb.StringValue{Value: "pong"})
			}})
		},
	}, procedure
}

func assertJSONRequestLog(t *testing.T, raw, method, path string, status int) {
	t.Helper()
	var rec map[string]any
	found := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("log line is not JSON: %q err=%v", line, err)
		}
		if item["msg"] == "http request" && item["path"] == path && item["method"] == method {
			rec = item
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing JSON request log for %s %s, got %s", method, path, raw)
	}
	got, ok := rec["status"].(float64)
	if !ok || int(got) != status {
		t.Fatalf("status = %v, want %d", rec["status"], status)
	}
	if rec["request_id"] == nil || rec["request_id"] == "" {
		t.Fatal("missing request_id")
	}
}
