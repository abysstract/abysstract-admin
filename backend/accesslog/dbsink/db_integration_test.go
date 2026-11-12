//go:build integration

package dbsink_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/accesslog/dbsink"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func TestDBSinkWriteQueryPurge(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sinks, err := accesslog.Open(ctx, []string{dbsink.Name}, accesslog.Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	sink := sinks[0].(*dbsink.Sink)
	defer sink.Close(ctx)
	if _, ok := sinks[0].(accesslog.Reader); !ok {
		t.Fatal("db sink must implement Reader")
	}

	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	entries := []accesslog.Entry{
		{
			Time: base, RequestID: "req-1", Kind: accesslog.KindRPC,
			Service: "abysstract.admin.v1.UserService", Procedure: "/abysstract.admin.v1.UserService/ListUsers",
			Method: "POST", Path: "/abysstract.admin.v1.UserService/ListUsers",
			Principal: accesslog.Principal{Type: accesslog.PrincipalUser, ID: "1", Name: "admin"},
			ClientIP:  "127.0.0.1", UserAgent: "test", Code: "ok", Status: 200,
			Duration: 1234 * time.Microsecond,
		},
		{
			Time: base.Add(time.Minute), RequestID: "req-2", Kind: accesslog.KindRPC,
			Service: "sync.v1.SyncService", Procedure: "/sync.v1.SyncService/PushItems",
			Principal: accesslog.Principal{Type: accesslog.PrincipalAPICredential, ID: "key-9", Name: "warehouse"},
			Code:      "invalid_argument", Error: "2 items rejected",
			Duration: 5 * time.Millisecond,
			Attrs:    map[string]any{"items_ok": 8, "items_failed": 2, "batch": "b_%1"},
		},
		{
			Time: base.Add(2 * time.Minute), RequestID: "req-3", Kind: accesslog.KindHTTP,
			Method: "GET", Path: "/static/avatar/a_b.png", Status: 404, Error: "Not Found",
			UserAgent: strings.Repeat("u", 400), // longer than the column
		},
	}
	if err := sink.Write(ctx, entries); err != nil {
		t.Fatal(err)
	}

	query := func(q accesslog.Query) accesslog.Page {
		t.Helper()
		page, err := sink.Query(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	ids := func(page accesslog.Page) string {
		var out []string
		for _, e := range page.Entries {
			out = append(out, e.RequestID)
		}
		return strings.Join(out, ",")
	}

	all := query(accesslog.Query{})
	if all.Total != 3 || ids(all) != "req-3,req-2,req-1" {
		t.Fatalf("all = %d %s", all.Total, ids(all))
	}
	got := all.Entries[1]
	if got.ID == "" || !got.Time.Equal(base.Add(time.Minute)) || got.Service != "sync.v1.SyncService" ||
		got.Principal != entries[1].Principal || got.Code != "invalid_argument" || got.Error != "2 items rejected" ||
		got.Duration != 5*time.Millisecond || !got.Failed() {
		t.Fatalf("round trip = %+v", got)
	}
	if fmt.Sprint(got.Attrs["items_ok"]) != "8" || fmt.Sprint(got.Attrs["items_failed"]) != "2" || got.Attrs["batch"] != "b_%1" {
		t.Fatalf("attrs = %#v", got.Attrs)
	}
	if first := all.Entries[2]; first.Attrs != nil || first.Duration != 1234*time.Microsecond || first.ClientIP != "127.0.0.1" {
		t.Fatalf("first = %+v", first)
	}
	if ua := all.Entries[0].UserAgent; len(ua) != 255 {
		t.Fatalf("user agent length = %d, want truncated to 255", len(ua))
	}

	cases := []struct {
		name string
		q    accesslog.Query
		want string
	}{
		{"time range", accesslog.Query{From: base.Add(30 * time.Second), To: base.Add(2 * time.Minute)}, "req-2"},
		{"principal id", accesslog.Query{Principal: "key-9"}, "req-2"},
		{"principal name", accesslog.Query{Principal: "admin"}, "req-1"},
		{"principal type", accesslog.Query{PrincipalType: accesslog.PrincipalUser}, "req-1"},
		{"procedure substring", accesslog.Query{Procedure: "PushItems"}, "req-2"},
		{"procedure prefix", accesslog.Query{Procedure: "/abysstract.admin.v1."}, "req-1"},
		{"path substring", accesslog.Query{Procedure: "a_b"}, "req-3"},
		{"like metacharacters are literal", accesslog.Query{Procedure: "%"}, ""},
		{"success", accesslog.Query{Result: accesslog.ResultSuccess}, "req-1"},
		{"failure", accesslog.Query{Result: accesslog.ResultFailure}, "req-3,req-2"},
		{"code", accesslog.Query{Code: "invalid_argument"}, "req-2"},
		{"request id", accesslog.Query{RequestID: "req-3"}, "req-3"},
		{"page", accesslog.Query{Offset: 1, Limit: 1}, "req-2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := query(c.q)
			if ids(page) != c.want {
				t.Fatalf("got %q, want %q", ids(page), c.want)
			}
			if c.name == "page" && page.Total != 3 {
				t.Fatalf("total = %d", page.Total)
			}
		})
	}

	deleted, err := sink.Purge(ctx, base.Add(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("purged %d, want 2", deleted)
	}
	if left := query(accesslog.Query{}); ids(left) != "req-3" {
		t.Fatalf("after purge = %s", ids(left))
	}
}

func TestDBSinkRetentionLoop(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	writer := dbsink.New(db, dbsink.Options{})
	if err := writer.Write(ctx, []accesslog.Entry{
		{Time: now.Add(-31 * 24 * time.Hour), RequestID: "old", Code: "ok"},
		{Time: now.Add(-time.Hour), RequestID: "new", Code: "ok"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// The first purge runs when the sink starts.
	sink := dbsink.New(db, dbsink.Options{Retention: 30 * 24 * time.Hour, PurgeInterval: time.Hour})
	deadline := time.Now().Add(10 * time.Second)
	for {
		page, err := sink.Query(ctx, accesslog.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total == 1 && page.Entries[0].RequestID == "new" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expired entry not purged: %+v", page.Entries)
		}
		time.Sleep(20 * time.Millisecond)
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sink.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
}

func TestDBSinkRequiresTable(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if _, err := accesslog.Open(context.Background(), []string{dbsink.Name}, accesslog.Config{DB: db}); err == nil || !strings.Contains(err.Error(), "sys_access_log") {
		t.Fatalf("Open on an empty database = %v", err)
	}
	if _, err := accesslog.Open(context.Background(), []string{dbsink.Name}, accesslog.Config{}); err == nil {
		t.Fatal("Open without DB must fail")
	}
}
