package product_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1/productv1connect"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/system"
)

type testEnv struct {
	base     string
	authAPI  adminv1connect.AuthServiceClient
	users    adminv1connect.UserServiceClient
	depts    adminv1connect.DeptServiceClient
	products productv1connect.ProductServiceClient
}

func startApp(t *testing.T, db *gorm.DB, opts product.ModuleOptions) *testEnv {
	t.Helper()
	systemDef, err := system.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	productDef, err := product.NewModuleWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	cfg := producttest.TestConfig(t, producttest.CurrentDSN(t))
	app, err := adminapp.New(adminapp.Options{
		Config:  cfg,
		DB:      db,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Modules: []module.Definition{systemDef, productDef},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{Transport: transport}
	t.Cleanup(func() {
		// Release unused client dials before the server waits for their first request.
		transport.CloseIdleConnections()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("run = %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("shutdown timeout")
		}
	})
	base := "http://" + app.ListenerAddr()
	return &testEnv{
		base:     base,
		authAPI:  adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(client, base, connecthttp.WithReadMaxBytes(0)))),
		users:    adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(client, base, connecthttp.WithReadMaxBytes(0)))),
		depts:    adminv1connect.NewDeptServiceClient(connect.NewClient(connecthttp.NewTransport(client, base, connecthttp.WithReadMaxBytes(0)))),
		products: productv1connect.NewProductServiceClient(connect.NewClient(connecthttp.NewTransport(client, base, connecthttp.WithReadMaxBytes(0)))),
	}
}

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

func (e *testEnv) login(t *testing.T, username, password string) string {
	t.Helper()
	resp, err := e.authAPI.Login(context.Background(), &adminv1.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	if resp.GetToken() == "" {
		t.Fatal("empty token")
	}
	return resp.GetToken()
}

func authed(t *testing.T, token string) context.Context {
	t.Helper()
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Authorization", "Bearer "+token)
	return ctx
}

func createDept(t *testing.T, env *testEnv, token, name string) int64 {
	t.Helper()
	resp, err := env.depts.CreateDept(authed(t, token), &adminv1.CreateDeptRequest{
		ParentId: 1,
		DeptName: name,
		OrderNum: 2,
	})
	if err != nil {
		t.Fatalf("create dept: %v", err)
	}
	return resp.GetDept().GetId()
}

func createScopedUser(t *testing.T, env *testEnv, db *gorm.DB, username string, deptID int64, roleKey string) string {
	t.Helper()
	roleID := producttest.QueryInt64(t, db, `SELECT id FROM sys_role WHERE role_key = ? AND del_flag = 0`, roleKey)
	admin := env.login(t, "admin", "admin123")
	_, err := env.users.CreateUser(authed(t, admin), &adminv1.CreateUserRequest{
		Username: username,
		Password: "Passw0rd!",
		Nickname: username,
		DeptId:   deptID,
		RoleIds:  []int64{roleID},
	})
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	producttest.MustExec(t, db, `UPDATE sys_user SET pwd_reset_required = 0 WHERE username = ?`, username)
	return env.login(t, username, "Passw0rd!")
}

func createSelfAndDeptUser(t *testing.T, env *testEnv, db *gorm.DB, username string, deptID, extraDept int64) string {
	t.Helper()
	_ = extraDept
	admin := env.login(t, "admin", "admin123")
	var selfRoleID int64
	if err := db.Raw(`SELECT id FROM sys_role WHERE role_key = 'biz_product_self'`).Scan(&selfRoleID).Error; err != nil || selfRoleID == 0 {
		producttest.MustExec(t, db, `INSERT INTO sys_role (role_name, role_key, data_scope) VALUES ('商品本人', 'biz_product_self', 5)`)
		selfRoleID = producttest.QueryInt64(t, db, `SELECT id FROM sys_role WHERE role_key = 'biz_product_self'`)
		pageID := producttest.QueryInt64(t, db, `SELECT id FROM sys_menu WHERE component = 'biz/product/index'`)
		addID := producttest.QueryInt64(t, db, `SELECT id FROM sys_menu WHERE perms = 'biz:product:add' AND menu_type = 'F'`)
		listID := producttest.QueryInt64(t, db, `SELECT id FROM sys_menu WHERE perms = 'biz:product:list' AND menu_type = 'F'`)
		rows := []struct{ RoleID, MenuID int64 }{{selfRoleID, pageID}, {selfRoleID, addID}, {selfRoleID, listID}}
		if err := db.Table("sys_role_menu").Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	editorID := producttest.QueryInt64(t, db, `SELECT id FROM sys_role WHERE role_key = 'biz_product_editor'`)
	_, err := env.users.CreateUser(authed(t, admin), &adminv1.CreateUserRequest{
		Username: username,
		Password: "Passw0rd!",
		Nickname: username,
		DeptId:   deptID,
		RoleIds:  []int64{editorID, selfRoleID},
	})
	if err != nil {
		t.Fatalf("create union user: %v", err)
	}
	producttest.MustExec(t, db, `UPDATE sys_user SET pwd_reset_required = 0 WHERE username = ?`, username)
	return env.login(t, username, "Passw0rd!")
}
