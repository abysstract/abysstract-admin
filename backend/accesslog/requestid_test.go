package accesslog_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect/v2/connecthttp"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

var generatedID = regexp.MustCompile(`^[0-9a-f]{24}$`)

func TestValidRequestID(t *testing.T) {
	for _, id := range []string{
		"req-1234",
		"0b5e8f0c-6a3f-4f37-9a43-1c0e2b7d9f10",
		"trace:abc.def_ghi+jk/l=m@n",
		strings.Repeat("a", accesslog.MaxRequestIDLength),
	} {
		if !accesslog.ValidRequestID(id) {
			t.Errorf("ValidRequestID(%q) = false", id)
		}
	}
	for _, id := range []string{
		"",
		"short",
		strings.Repeat("a", accesslog.MaxRequestIDLength+1),
		"has space 123",
		"line\nbreak1234",
		"quote\"1234567",
		"ünïcode-12345",
	} {
		if accesslog.ValidRequestID(id) {
			t.Errorf("ValidRequestID(%q) = true", id)
		}
	}
}

func TestNewRequestID(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := accesslog.NewRequestID()
		if !generatedID.MatchString(id) || !accesslog.ValidRequestID(id) {
			t.Fatalf("NewRequestID() = %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate request ID %q", id)
		}
		seen[id] = true
	}
}

func TestMiddlewareAssignsRequestID(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	var seen []string
	h := accesslog.Middleware(l, accesslog.CaptureOptions{
		AssignRequestID: true,
		Skip:            accesslog.SkipPaths("/static/"),
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, accesslog.RequestIDFromContext(r.Context()))
	}))
	serve := func(path, id string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if id != "" {
			req.Header.Set(accesslog.HeaderRequestID, id)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Header().Get(accesslog.HeaderRequestID)
	}

	generated := serve("/a", "")
	kept := serve("/b", "client-req-0001")
	replaced := serve("/c", "bad id\x01")
	skipped := serve("/static/app.js", "")
	closeLogger(t, l)

	if !generatedID.MatchString(generated) || kept != "client-req-0001" || !generatedID.MatchString(replaced) || !generatedID.MatchString(skipped) {
		t.Fatalf("response headers = %q %q %q %q", generated, kept, replaced, skipped)
	}
	if strings.Join(seen, ",") != strings.Join([]string{generated, kept, replaced, skipped}, ",") {
		t.Fatalf("context IDs = %v", seen)
	}
	got := sink.entries()
	if len(got) != 3 {
		t.Fatalf("entries = %d, want 3 (static skipped)", len(got))
	}
	for i, want := range []string{generated, kept, replaced} {
		if got[i].RequestID != want {
			t.Fatalf("entry %d request ID = %q, want %q", i, got[i].RequestID, want)
		}
	}
}

func TestRequestIDNotAssignedByDefault(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	h := accesslog.Middleware(l, accesslog.CaptureOptions{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := accesslog.RequestIDFromContext(r.Context()); id != "" {
			t.Errorf("context request ID = %q", id)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(accesslog.HeaderRequestID, "client-req-0002")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	closeLogger(t, l)
	if got := rec.Header().Get(accesslog.HeaderRequestID); got != "" {
		t.Fatalf("response header = %q", got)
	}
	if got := sink.entries(); len(got) != 1 || got[0].RequestID != "client-req-0002" {
		t.Fatalf("entries = %+v", got)
	}
}

func TestRequestIDAssignedWithoutLogger(t *testing.T) {
	var disabled *accesslog.Logger
	opts := accesslog.CaptureOptions{AssignRequestID: true}
	var inHandler string
	h := accesslog.Middleware(disabled, opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inHandler = accesslog.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get(accesslog.HeaderRequestID); !generatedID.MatchString(got) || got != inHandler {
		t.Fatalf("header %q, context %q", got, inHandler)
	}

	mux := http.NewServeMux()
	connecthttp.Mount(mux, echoServer(t, accesslog.Interceptor(disabled, opts), authInterceptor()))
	res := call(t, mux, "hi", map[string]string{"X-Api-Key": "k"}, "")
	if got := res.Header().Get(accesslog.HeaderRequestID); !generatedID.MatchString(got) {
		t.Fatalf("interceptor response header = %q", got)
	}
}

func TestInterceptorAloneAssignsRequestID(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	opts := accesslog.CaptureOptions{AssignRequestID: true}
	mux := http.NewServeMux()
	connecthttp.Mount(mux, echoServer(t, accesslog.Interceptor(l, opts), authInterceptor()))

	generated := call(t, mux, "hi", map[string]string{"X-Api-Key": "k"}, "").Header().Get(accesslog.HeaderRequestID)
	kept := call(t, mux, "hi", map[string]string{"X-Api-Key": "k", "X-Request-Id": "client-req-0003"}, "").Header().Get(accesslog.HeaderRequestID)
	// Failed calls carry the header too.
	failed := call(t, mux, "hi", nil, "").Header().Get(accesslog.HeaderRequestID)
	closeLogger(t, l)

	if !generatedID.MatchString(generated) || kept != "client-req-0003" || !generatedID.MatchString(failed) {
		t.Fatalf("response headers = %q %q %q", generated, kept, failed)
	}
	got := sink.entries()
	if len(got) != 3 || got[0].RequestID != generated || got[1].RequestID != kept || got[2].RequestID != failed {
		t.Fatalf("entries = %+v", got)
	}
}

func TestMiddlewareAndInterceptorShareAssignedRequestID(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	opts := accesslog.CaptureOptions{AssignRequestID: true}
	mux := http.NewServeMux()
	connecthttp.Mount(mux, echoServer(t, accesslog.Interceptor(l, opts), authInterceptor()))
	res := call(t, accesslog.Middleware(l, opts)(mux), "hi", map[string]string{"X-Api-Key": "k"}, "")
	closeLogger(t, l)

	id := res.Header().Values(accesslog.HeaderRequestID)
	if len(id) != 1 || !generatedID.MatchString(id[0]) {
		t.Fatalf("response header = %q", id)
	}
	if got := sink.entries(); len(got) != 1 || got[0].RequestID != id[0] || got[0].Kind != accesslog.KindRPC {
		t.Fatalf("entries = %+v", got)
	}
}

func TestSkipHelpers(t *testing.T) {
	skip := accesslog.SkipPaths("/static/", "/ping", "")
	cases := []struct {
		method, path      string
		defaultSkip, skip bool
	}{
		{http.MethodGet, "/healthz", true, true},
		{http.MethodGet, "/readyz", true, true},
		{http.MethodGet, "/metrics", true, true},
		{http.MethodGet, "/version", true, true},
		{http.MethodOptions, "/sync.v1.SyncService/Push", true, true},
		{http.MethodPost, "/sync.v1.SyncService/Push", false, false},
		{http.MethodGet, "/static/app.js", false, true},
		{http.MethodGet, "/static", false, false},
		{http.MethodGet, "/ping", false, true},
		{http.MethodGet, "/ping/x", false, false},
		{http.MethodGet, "/", false, false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, nil)
		if got := accesslog.DefaultSkip(r); got != c.defaultSkip {
			t.Errorf("DefaultSkip(%s %s) = %v", c.method, c.path, got)
		}
		if got := skip(r); got != c.skip {
			t.Errorf("SkipPaths(%s %s) = %v", c.method, c.path, got)
		}
	}
}
