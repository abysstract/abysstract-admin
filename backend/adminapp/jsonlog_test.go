//go:build integration

package adminapp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestJSONRequestLogRecordsHTTPFields(t *testing.T) {
	skipSystemTables(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	app, err := New(Options{
		Config:  testConfig(t),
		Logger:  logger,
		DB:      sqliteDB(t),
		Modules: []module.Definition{systemModule()},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	resp, err := http.Get("http://" + app.ListenerAddr() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}

	var rec map[string]any
	found := false
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("log line is not JSON: %q err=%v", line, err)
		}
		if item["msg"] == "http request" && item["path"] == "/healthz" {
			rec = item
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing http request JSON log, got %s", buf.String())
	}
	if rec["method"] != "GET" {
		t.Fatalf("method = %v", rec["method"])
	}
	status, ok := rec["status"].(float64)
	if !ok || status != 200 {
		t.Fatalf("status = %v", rec["status"])
	}
	if rec["request_id"] == nil || rec["request_id"] == "" {
		t.Fatal("missing request_id")
	}
	if rec["level"] != "INFO" {
		t.Fatalf("level = %v", rec["level"])
	}
}
