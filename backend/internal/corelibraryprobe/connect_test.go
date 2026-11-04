package corelibraryprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/module"
)

const (
	probeServicePath = "/company.contract.v1.ProbeService/"
	probeProcedure   = probeServicePath + "Ping"
)

func probeService(t *testing.T, onCall ...func()) (module.ConnectService, protoreflect.MethodDescriptor) {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("company/contract/v1/probe.proto"),
		Package:    proto.String("company.contract.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/wrappers.proto"},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("ProbeService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("Ping"),
				InputType:  proto.String(".google.protobuf.StringValue"),
				OutputType: proto.String(".google.protobuf.StringValue"),
			}},
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("build probe descriptor: %v", err)
	}
	service := file.Services().ByName("ProbeService")
	if service == nil {
		t.Fatal("probe service descriptor is missing")
	}
	method := service.Methods().ByName("Ping")
	if method == nil {
		t.Fatal("probe method descriptor is missing")
	}
	if got := "/" + string(service.FullName()) + "/" + string(method.Name()); got != probeProcedure {
		t.Fatalf("procedure = %q, want %q", got, probeProcedure)
	}
	return module.ConnectService{
		Descriptor: service,
		Register: func(server *connect.Server) {
			server.Register(connect.Method{Spec: connect.Spec{Procedure: probeProcedure, Schema: method}, Handler: func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				var req wrapperspb.StringValue
				if err := stream.Receive(&req); err != nil {
					return err
				}
				if len(onCall) > 0 && onCall[0] != nil {
					onCall[0]()
				}
				return stream.Send(&wrapperspb.StringValue{Value: "pong"})
			}})
		},
	}, method
}

func TestConnectServiceDescriptorAndOptions(t *testing.T) {
	var handlerCalls atomic.Int32
	service, method := probeService(t, func() { handlerCalls.Add(1) })
	if service.Descriptor.FullName() != "company.contract.v1.ProbeService" {
		t.Fatalf("service descriptor = %q", service.Descriptor.FullName())
	}
	if method.FullName() != "company.contract.v1.ProbeService.Ping" {
		t.Fatalf("method descriptor = %q", method.FullName())
	}

	var interceptorCalls atomic.Int32
	var denied atomic.Bool
	interceptor := connect.ServerInterceptor(func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			interceptorCalls.Add(1)
			if denied.Load() {
				return connect.NewError(connect.CodePermissionDenied, "probe denied")
			}
			return next(ctx, spec, stream)
		}
	})

	rpcServer := connect.NewServer(interceptor)
	service.Register(rpcServer)
	mux := http.NewServeMux()
	connecthttp.Mount(mux, rpcServer, connecthttp.WithReadMaxBytes(64))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL))

	response := &wrapperspb.StringValue{}
	err := client.CallUnary(context.Background(), connect.Spec{Procedure: probeProcedure}, &wrapperspb.StringValue{Value: "ping"}, response)
	if err != nil {
		t.Fatalf("allowed Connect call: %v", err)
	}
	if response.Value != "pong" || handlerCalls.Load() != 1 || interceptorCalls.Load() != 1 {
		t.Fatalf("allowed call response=%q handlerCalls=%d interceptorCalls=%d", response.Value, handlerCalls.Load(), interceptorCalls.Load())
	}

	denied.Store(true)
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: probeProcedure}, &wrapperspb.StringValue{Value: "denied"}, &wrapperspb.StringValue{})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("denied call code = %s, want permission denied (err=%v)", connect.CodeOf(err), err)
	}
	if handlerCalls.Load() != 1 {
		t.Fatal("permission denial must not call the business handler")
	}

	denied.Store(false)
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: probeProcedure}, &wrapperspb.StringValue{Value: strings.Repeat("x", 1024)}, &wrapperspb.StringValue{})
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("oversized request code = %s, want resource exhausted (err=%v)", connect.CodeOf(err), err)
	}
	if handlerCalls.Load() != 1 {
		t.Fatal("oversized request must not call the business handler")
	}
}
