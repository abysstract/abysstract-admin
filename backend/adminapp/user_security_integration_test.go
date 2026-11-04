//go:build integration

package adminapp

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestSystemUserAuthAndBusinessDenial(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	svc, procedure := probeService(t, "company.contract.v1", "BizService", func() { calls.Add(1) })
	app, err := New(Options{
		Config: testConfig(t),
		Logger: silentLogger(),
		DB:     db,
		Modules: []module.Definition{
			{
				Name:   "system",
				Create: emptyCreate,
			},
			{
				Name:     "billing",
				Requires: []string{"system"},
				Services: []module.ConnectService{svc},
				HTTP: []module.HTTPRoute{{
					Name: "biz", Method: "POST", Path: "/biz/run", Access: module.AccessPermission, Permission: "biz:contract:list",
				}},
				Permissions: generatedPermissions(procedure, "biz:contract:list"),
				Create: func(context.Context, module.Runtime) (module.Instance, error) {
					return module.Instance{
						Services: []module.ConnectService{svc},
						HTTP: []module.HTTPHandler{{Name: "biz", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							calls.Add(1)
							w.WriteHeader(http.StatusNoContent)
						})}},
					}, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	base := "http://" + app.ListenerAddr()
	if err := db.Exec(`
		INSERT INTO sys_user (dept_id, username, nickname, email, password, status, remark)
		VALUES (1, 'alice', 'Alice', 'alice@example.com', 'x', 1, 'tester')`).Error; err != nil {
		t.Fatal(err)
	}
	var aliceID int64
	if err := db.Raw("SELECT id FROM sys_user WHERE username = ?", "alice").Scan(&aliceID).Error; err != nil {
		t.Fatal(err)
	}

	client := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: procedure}, &wrapperspb.StringValue{Value: "ping"}, &wrapperspb.StringValue{})
	if err == nil {
		t.Fatal("anonymous connect should fail")
	}
	if calls.Load() != 0 {
		t.Fatal("anonymous connect invoked handler")
	}

	mgr := jwt.NewManager("test-secret-not-default", time.Hour)
	call := func(userID int64, username string) error {
		var ver int64
		if err := db.Raw("SELECT token_version FROM sys_user WHERE id = ?", userID).Scan(&ver).Error; err != nil {
			return err
		}
		token, _, err := mgr.Generate(userID, username, ver)
		if err != nil {
			return err
		}
		req := &wrapperspb.StringValue{Value: "ping"}
		reqCtx, reqInfo := connect.NewClientContext(context.Background())
		reqInfo.RequestHeader().Set("Authorization", "Bearer "+token)
		err = client.CallUnary(reqCtx, connect.Spec{Procedure: procedure}, req, &wrapperspb.StringValue{})
		return err
	}
	if err := call(aliceID, "alice"); err == nil {
		t.Fatal("alice without permission should be denied")
	}
	if calls.Load() != 0 {
		t.Fatal("alice invoked handler")
	}
	if err := call(1, "admin"); err != nil {
		t.Fatalf("admin wildcard: %v", err)
	}
	if calls.Load() == 0 {
		t.Fatal("admin wildcard did not invoke handler")
	}

	httpReq, _ := http.NewRequest(http.MethodPost, base+"/biz/run", strings.NewReader(`{"operator_id":99}`))
	var aliceVer int64
	if err := db.Raw("SELECT token_version FROM sys_user WHERE id = ?", aliceID).Scan(&aliceVer).Error; err != nil {
		t.Fatal(err)
	}
	aliceToken, _, err := mgr.Generate(2, "alice", aliceVer)
	if err != nil {
		t.Fatal(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("alice http status = %d", resp.StatusCode)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}
}
