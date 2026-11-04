//go:build integration

package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
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

// Exercise the seven migrated updates through production assembly and HTTP.
func TestProductionUpdateRPCs(t *testing.T) {
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
		"sys_config":    {"config_name": "UpdateRPC config", "config_key": "updatecaserpc", "config_value": "keep", "remark": "keep"},
		"sys_dept":      {"dept_name": "UpdateRPC dept", "order_num": 7},
		"sys_dict_type": {"dict_name": "UpdateRPC type", "dict_type": "updatecaserpc", "remark": "keep"},
		"sys_dict_data": {"dict_label": "UpdateRPC label", "dict_type": "updatecaserpc", "dict_value": "updatecaserpc", "is_default": 1},
		"sys_menu":      {"menu_name": "UpdateRPC menu", "menu_type": "C", "visible": 1},
		"sys_notice":    {"notice_title": "UpdateRPC notice", "notice_type": 1, "notice_content": "keep", "remark": "keep"},
		"sys_role":      {"role_name": "UpdateRPC role", "role_key": "updatecaserpc", "role_sort": 7},
	} {
		row["id"] = 9902
		if err := db.Table(table).Create(row).Error; err != nil {
			t.Fatal(err)
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
	if err := db.Exec("INSERT INTO sys_user (id,username,password,status) VALUES (9901,'update-denied','x',1)").Error; err != nil {
		t.Fatal(err)
	}
	denied, _, err := jwt.NewManager(cfg.JWTSecret, time.Hour).Generate(9901, "update-denied", 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ procedure, table, column, entity, patch string }{
		{adminv1connect.ConfigServiceUpdateConfigProcedure, "sys_config", "remark", "item", `"remark":""`},
		{adminv1connect.DeptServiceUpdateDeptProcedure, "sys_dept", "order_num", "dept", `"orderNum":0`},
		{adminv1connect.DictServiceUpdateDictTypeProcedure, "sys_dict_type", "remark", "item", `"remark":""`},
		{adminv1connect.DictServiceUpdateDictDataProcedure, "sys_dict_data", "is_default", "item", `"isDefault":0`},
		{adminv1connect.MenuServiceUpdateMenuProcedure, "sys_menu", "visible", "menu", `"visible":0`},
		{adminv1connect.NoticeServiceUpdateNoticeProcedure, "sys_notice", "remark", "item", `"remark":""`},
		{adminv1connect.RoleServiceUpdateRoleProcedure, "sys_role", "role_sort", "role", `"roleSort":0`},
	}
	for _, c := range cases {
		t.Run(c.table, func(t *testing.T) {
			var before map[string]any
			if err := db.Table(c.table).Where("id = ?", 9902).Take(&before).Error; err != nil {
				t.Fatal(err)
			}
			if reflect.ValueOf(before[c.column]).IsZero() {
				t.Fatal("nonzero fixture required")
			}
			audit := before["update_time"].(time.Time)
			for _, tc := range []struct {
				name, token, patch string
				status             int
			}{
				{"anonymous", "", c.patch, http.StatusUnauthorized},
				{"denied", denied, c.patch, http.StatusForbidden},
				{"empty", login.GetToken(), "", http.StatusBadRequest},
				{"update", login.GetToken(), c.patch, http.StatusOK},
				{"same-value", login.GetToken(), c.patch, http.StatusOK},
			} {
				t.Run(tc.name, func(t *testing.T) {
					body := `{"id":"9902"}`
					if tc.patch != "" {
						body = `{"id":"9902",` + tc.patch + `}`
					}
					req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+c.procedure, strings.NewReader(body))
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
					data, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != tc.status {
						t.Fatalf("status=%d body=%s", resp.StatusCode, data)
					}
					var saved map[string]any
					if err := db.Table(c.table).Where("id = ?", 9902).Take(&saved).Error; err != nil {
						t.Fatal(err)
					}
					gotAudit := saved["update_time"].(time.Time)
					if tc.name == "update" {
						if !gotAudit.After(audit) {
							t.Fatal("update must advance audit time")
						}
						before[c.column] = saved[c.column]
						before["update_time"] = saved["update_time"]
						if !reflect.ValueOf(saved[c.column]).IsZero() {
							t.Fatalf("zero value: %+v", saved)
						}
						audit = gotAudit
					}
					if !reflect.DeepEqual(saved, before) {
						t.Fatalf("unexpected changed columns: before=%+v saved=%+v", before, saved)
					}
					if tc.status == http.StatusOK {
						var payload map[string]struct {
							UpdateTime string `json:"updateTime"`
						}
						if err := json.Unmarshal(data, &payload); err != nil {
							t.Fatal(err)
						}
						returned, err := time.Parse(time.RFC3339Nano, payload[c.entity].UpdateTime)
						if err != nil {
							t.Fatal(err)
						}
						if !returned.Equal(audit) {
							t.Fatalf("response time=%v database=%v", returned, audit)
						}
					}
				})
			}
		})
	}
}
