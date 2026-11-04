package adminapp

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func testConfig(t *testing.T) adminconfig.Config {
	t.Helper()
	return adminconfig.Config{
		Env:      adminconfig.EnvDev,
		Addr:     "127.0.0.1:0",
		DBDriver: "postgres", DBDSN: "host=localhost user=test password=test dbname=test sslmode=disable",
		JWTSecret:           "test-secret-not-default",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"http://localhost:5173"},
		ShutdownTimeout:     5 * time.Second,
		DBMaxOpenConns:      5,
		DBMaxIdleConns:      2,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
		UploadDir:           t.TempDir(),
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func emptyCreate(context.Context, module.Runtime) (module.Instance, error) {
	return module.Instance{}, nil
}

func systemModule() module.Definition {
	return module.Definition{Name: "system", Create: emptyCreate}
}

func countingCreate(n *atomic.Int32) func(context.Context, module.Runtime) (module.Instance, error) {
	return func(context.Context, module.Runtime) (module.Instance, error) {
		n.Add(1)
		return module.Instance{}, nil
	}
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

func publicPermissions(procedure string) func(permission.Registrar) error {
	return func(reg permission.Registrar) error {
		return reg.RegisterPublic(procedure)
	}
}

func generatedPermissions(procedure, perm string) func(permission.Registrar) error {
	return func(reg permission.Registrar) error {
		return reg.RegisterGenerated(permission.Entry{Procedure: procedure, Permission: perm})
	}
}

func skipSystemTables(t *testing.T) {
	t.Helper()
	restore := OverrideRequireSystemTables(func(*gorm.DB) error { return nil })
	t.Cleanup(restore)
}

func seedAuthSchema(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE sys_user (id INTEGER PRIMARY KEY, token_version INTEGER, pwd_reset_required INTEGER, del_flag INTEGER)`,
		`CREATE TABLE sys_role (id INTEGER PRIMARY KEY, role_key TEXT, status INTEGER, del_flag INTEGER)`,
		`CREATE TABLE sys_user_role (user_id INTEGER, role_id INTEGER)`,
		`CREATE TABLE sys_menu (id INTEGER PRIMARY KEY, perms TEXT, status INTEGER)`,
		`CREATE TABLE sys_role_menu (role_id INTEGER, menu_id INTEGER)`,
		`CREATE TABLE sys_oper_log (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, method TEXT, oper_name TEXT, oper_id INTEGER, oper_ip TEXT, status INTEGER, error_msg TEXT, cost_time INTEGER, oper_time DATETIME, oper_param TEXT)`,
		`INSERT INTO sys_user (id, token_version, pwd_reset_required, del_flag) VALUES (1, 0, 0, 0)`,
		`INSERT INTO sys_user (id, token_version, pwd_reset_required, del_flag) VALUES (2, 0, 1, 0)`,
		`INSERT INTO sys_role (id, role_key, status, del_flag) VALUES (1, 'user', 1, 0)`,
		`INSERT INTO sys_user_role (user_id, role_id) VALUES (1, 1)`,
	}
	for _, stmt := range stmts {
		if err := gdb.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}
