package stdoutsink_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/accesslog/stdoutsink"
)

func TestStdoutLineFormat(t *testing.T) {
	var buf bytes.Buffer
	sinks, err := accesslog.Open(context.Background(), []string{"stdout"}, accesslog.Config{Output: &buf})
	if err != nil {
		t.Fatal(err)
	}
	sink := sinks[0]
	start := time.Date(2026, 11, 10, 8, 30, 0, 123456789, time.FixedZone("CST", 8*3600))
	err = sink.Write(context.Background(), []accesslog.Entry{
		{
			Time: start, RequestID: "req-1", Kind: accesslog.KindRPC,
			Service: "sync.v1.SyncService", Procedure: "/sync.v1.SyncService/PushItems",
			Method: "POST", Path: "/sync.v1.SyncService/PushItems",
			Principal: accesslog.Principal{Type: accesslog.PrincipalAPICredential, ID: "key-9", Name: "warehouse"},
			ClientIP:  "203.0.113.9", UserAgent: "sync/1.0",
			Code: "ok", Status: 200, Duration: 1234567 * time.Nanosecond,
			Attrs: map[string]any{"items_ok": 8, "items_failed": 0},
		},
		{Time: start, Kind: accesslog.KindHTTP, Method: "GET", Path: "/x", Status: 500, Error: "boom"},
		{Time: start, Kind: accesslog.KindHTTP, Attrs: map[string]any{"bad": func() {}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %q", buf.String())
	}

	want := `{"type":"access_log","time":"2026-11-10T00:30:00.123456789Z","request_id":"req-1","kind":"rpc",` +
		`"service":"sync.v1.SyncService","procedure":"/sync.v1.SyncService/PushItems","method":"POST",` +
		`"path":"/sync.v1.SyncService/PushItems","principal_type":"api_credential","principal_id":"key-9",` +
		`"principal_name":"warehouse","client_ip":"203.0.113.9","user_agent":"sync/1.0","code":"ok","status":200,` +
		`"success":true,"duration_ms":1.235,"attrs":{"items_failed":0,"items_ok":8}}`
	if lines[0] != want {
		t.Fatalf("line =\n%s\nwant\n%s", lines[0], want)
	}

	var failed map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &failed); err != nil {
		t.Fatal(err)
	}
	if failed["success"] != false || failed["error"] != "boom" || failed["code"] != "" {
		t.Fatalf("failed line = %v", failed)
	}
	if _, ok := failed["attrs"]; ok {
		t.Fatal("empty attrs must be omitted")
	}
	// Every documented field is present even when empty.
	for _, key := range []string{"type", "time", "request_id", "kind", "service", "procedure", "method", "path",
		"principal_type", "principal_id", "principal_name", "client_ip", "user_agent", "code", "status", "success", "duration_ms"} {
		if _, ok := failed[key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}

	if !strings.Contains(lines[2], `"_encode_error"`) {
		t.Fatalf("unencodable attrs line = %s", lines[2])
	}
}

func TestStdoutName(t *testing.T) {
	if got := stdoutsink.New(nil).Name(); got != "stdout" {
		t.Fatal(got)
	}
}
