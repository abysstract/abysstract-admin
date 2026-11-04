//go:build integration

package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/module"
)

// Exercise all eight lists through the production module, HTTP codec, auth and
// permission interceptors, including injected repositories and cache wrappers.
func TestProductionListRPCs(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	app, err := adminapp.New(adminapp.Options{Config: cfg, DB: db, Logger: silentLogger(), Modules: []module.Definition{def}})
	if err != nil {
		t.Fatal(err)
	}

	for table, row := range map[string]map[string]any{
		"sys_config":    {"config_name": "ListCaseRPC config", "config_key": "listcaserpc"},
		"sys_dept":      {"dept_name": "ListCaseRPC dept"},
		"sys_dict_type": {"dict_name": "ListCaseRPC type", "dict_type": "listcaserpc"},
		"sys_dict_data": {"dict_label": "ListCaseRPC label", "dict_type": "listcaserpc", "dict_value": "listcaserpc"},
		"sys_menu":      {"menu_name": "ListCaseRPC menu", "menu_type": "C"},
		"sys_notice":    {"notice_title": "ListCaseRPC notice", "notice_type": 1},
		"sys_role":      {"role_name": "ListCaseRPC role", "role_key": "listcaserpc"},
		"sys_user":      {"username": "ListCaseRPC user", "dept_id": 1, "password": "x"},
	} {
		deleted := make(map[string]any, len(row)+2)
		for k, v := range row {
			deleted[k] = v
		}
		row["id"] = 9902
		if err := db.Table(table).Create(row).Error; err != nil {
			t.Fatal(err)
		}
		if table == "sys_dept" || table == "sys_role" || table == "sys_user" {
			deleted["id"] = 9903
			deleted["del_flag"] = 1
			// Activity uniqueness permits a deleted copy of the same business key.
			if err := db.Table(table).Create(deleted).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	waitListening(t, app, done)
	base := "http://" + app.ListenerAddr()
	login, err := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))).Login(ctx, &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sys_user (id,username,password,status) VALUES (9901,'list-denied','x',1)").Error; err != nil {
		t.Fatal(err)
	}
	denied, _, err := jwt.NewManager(cfg.JWTSecret, time.Hour).Generate(9901, "list-denied", 0)
	if err != nil {
		t.Fatal(err)
	}
	procedures := []string{
		adminv1connect.ConfigServiceListConfigsProcedure,
		adminv1connect.DeptServiceListDeptRowsProcedure,
		adminv1connect.DictServiceListDictTypesProcedure,
		adminv1connect.DictServiceListDictDataProcedure,
		adminv1connect.MenuServiceListMenusProcedure,
		adminv1connect.NoticeServiceListNoticesProcedure,
		adminv1connect.RoleServiceListRolesProcedure,
		adminv1connect.UserServiceListUsersProcedure,
	}
	for _, procedure := range procedures {
		t.Run(procedure, func(t *testing.T) {
			for _, tc := range []struct {
				name, token string
				status      int
			}{{"anonymous", "", http.StatusUnauthorized}, {"denied", denied, http.StatusForbidden}, {"admin", login.GetToken(), http.StatusOK}, {"admin-repeat", login.GetToken(), http.StatusOK}} {
				t.Run(tc.name, func(t *testing.T) {
					// The repeated admin call also exercises production list caching.
					req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+procedure, strings.NewReader(`{"keyword":"ListCaseRPC"}`))
					if err != nil {
						t.Fatal(err)
					}
					req.Header.Set("Content-Type", "application/json")
					if tc.token != "" {
						req.Header.Set("Authorization", "Bearer "+tc.token)
					}
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != tc.status {
						t.Fatalf("status=%d body=%s", resp.StatusCode, body)
					}

					if tc.status == http.StatusOK {
						var payload struct {
							Items []json.RawMessage `json:"items"`
							Page  struct {
								Total string `json:"total"`
							} `json:"page"`
						}
						if err := json.Unmarshal(body, &payload); err != nil {
							t.Fatal(err)
						}
						if payload.Page.Total != "1" || len(payload.Items) != 1 {
							t.Fatalf("expected one active matching row: %s", body)
						}
					}
				})
			}
		})
	}
}
