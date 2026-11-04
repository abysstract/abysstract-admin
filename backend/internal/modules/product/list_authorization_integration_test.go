//go:build integration

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
	"golang.org/x/crypto/bcrypt"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1/productv1connect"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/system"
)

// RPC permission enforcement precedes the controller; repository tests supply AccessScope.
func TestProductionListAuthorizationBoundary(t *testing.T) {
	db := producttest.OpenMigrated(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	producttest.MustExec(t, db, `INSERT INTO sys_user(username,password,nickname,dept_id,pwd_reset_required) VALUES('list_no_permission',?,'无列表权限',1,0)`, string(hash))
	sys, err := system.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	biz, err := product.NewModule()
	if err != nil {
		t.Fatal(err)
	}
	app, err := adminapp.New(adminapp.Options{Config: producttest.TestConfig(t, producttest.CurrentDSN(t)), DB: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Modules: []module.Definition{sys, biz}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("server shutdown timeout")
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	for app.ListenerAddr() == "" {
		if time.Now().After(deadline) {
			t.Fatal("listener timeout")
		}
		select {
		case err := <-done:
			t.Fatalf("server stopped: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	base := "http://" + app.ListenerAddr()
	auth := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base)))
	products := productv1connect.NewProductServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base)))
	if _, err := products.ListProducts(context.Background(), &productv1.ListProductsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous: %v", err)
	}
	for _, tc := range []struct {
		user, password string
		allowed        bool
	}{{"admin", "admin123", true}, {"list_no_permission", "Passw0rd!", false}} {
		login, err := auth.Login(context.Background(), &adminv1.LoginRequest{Username: tc.user, Password: tc.password})
		if err != nil {
			t.Fatal(err)
		}
		request := &productv1.ListProductsRequest{}
		requestCtx, requestInfo := connect.NewClientContext(context.Background())
		requestInfo.RequestHeader().Set("Authorization", "Bearer "+login.GetToken())
		_, err = products.ListProducts(requestCtx, request)
		if tc.allowed {
			if err != nil {
				t.Fatalf("allowed: %v", err)
			}
		} else if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("no list permission: %v", err)
		}
		if tc.allowed {
			invalid := &productv1.ListProductsRequest{Orders: []*adminv1.OrderBy{{Field: "missing_column"}}}
			invalidCtx, invalidInfo := connect.NewClientContext(context.Background())
			invalidInfo.RequestHeader().Set("Authorization", "Bearer "+login.GetToken())
			if _, err := products.ListProducts(invalidCtx, invalid); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("controller sort whitelist: %v", err)
			}
			invalidPage := &productv1.ListProductsRequest{Page: &adminv1.PageRequest{PageSize: -1}}
			invalidPageCtx, invalidPageInfo := connect.NewClientContext(context.Background())
			invalidPageInfo.RequestHeader().Set("Authorization", "Bearer "+login.GetToken())
			if _, err := products.ListProducts(invalidPageCtx, invalidPage); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("controller page validation: %v", err)
			}
		}
	}
}
