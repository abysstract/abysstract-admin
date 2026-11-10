package accesslog_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

// fixedClock advances by step on every call so durations are deterministic.
func fixedClock(step time.Duration) func() time.Time {
	now := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	return func() time.Time {
		t := now
		now = now.Add(step)
		return t
	}
}

const testProcedure = "/test.v1.EchoService/Echo"

// echoServer serves one unary procedure. The handler fails with
// PermissionDenied when the request value is "deny".
func echoServer(t *testing.T, interceptors ...connect.ServerInterceptor) *connect.Server {
	t.Helper()
	server := connect.NewServer(interceptors...)
	server.Register(unary(func(ctx context.Context, req *wrapperspb.StringValue) (*emptypb.Empty, error) {
		accesslog.SetAttr(ctx, "items_ok", 3)
		accesslog.AddAttrs(ctx, map[string]any{"items_failed": 1})
		if req.GetValue() == "deny" {
			return nil, connect.NewError(connect.CodePermissionDenied, "no access")
		}
		return &emptypb.Empty{}, nil
	}))
	return server
}

func unary(fn func(context.Context, *wrapperspb.StringValue) (*emptypb.Empty, error)) connect.Method {
	return connect.Method{
		Spec: connect.Spec{Procedure: testProcedure, StreamType: connect.StreamTypeUnary},
		Handler: func(ctx context.Context, _ connect.Spec, stream connect.ServerStream) error {
			var req wrapperspb.StringValue
			if err := stream.Receive(&req); err != nil {
				return err
			}
			res, err := fn(ctx, &req)
			if err != nil {
				return err
			}
			return stream.Send(res)
		},
	}
}

// authInterceptor stands in for an application's credential check.
func authInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			info, _ := connect.CallInfoForServerContext(ctx)
			key := info.RequestHeader().Get("X-Api-Key")
			if key == "" {
				return connect.NewError(connect.CodeUnauthenticated, "missing api key")
			}
			accesslog.SetPrincipal(ctx, accesslog.Principal{Type: accesslog.PrincipalAPICredential, ID: "k-1", Name: key})
			return next(ctx, spec, stream)
		}
	}
}

func call(t *testing.T, handler http.Handler, value string, header map[string]string, remote string) *httptest.ResponseRecorder {
	t.Helper()
	body := `"` + value + `"`
	req := httptest.NewRequest(http.MethodPost, testProcedure, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestInterceptorAloneCapturesConnectCall(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	opts := accesslog.CaptureOptions{Now: fixedClock(1500 * time.Microsecond)}
	server := echoServer(t, accesslog.Interceptor(l, opts), authInterceptor())
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)

	rec := call(t, mux, "hi", map[string]string{"X-Api-Key": "sync-client", "X-Request-Id": "req-1234567890abcdef", "User-Agent": "sync/1.0"}, "10.0.0.5:4444")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
	call(t, mux, "x", nil, "10.0.0.6:4444")
	call(t, mux, "deny", map[string]string{"X-Api-Key": "sync-client"}, "10.0.0.5:4444")
	closeLogger(t, l)

	got := sink.entries()
	if len(got) != 3 {
		t.Fatalf("entries = %d", len(got))
	}
	ok := got[0]
	if ok.Kind != accesslog.KindRPC || ok.Procedure != testProcedure || ok.Service != "test.v1.EchoService" {
		t.Fatalf("rpc fields = %+v", ok)
	}
	if ok.Method != http.MethodPost || ok.Path != testProcedure {
		t.Fatalf("method/path = %q %q", ok.Method, ok.Path)
	}
	if ok.Principal != (accesslog.Principal{Type: "api_credential", ID: "k-1", Name: "sync-client"}) {
		t.Fatalf("principal = %+v", ok.Principal)
	}
	if ok.Code != "ok" || ok.Error != "" || ok.Status != 0 || ok.Failed() {
		t.Fatalf("result = %+v", ok)
	}
	if ok.ClientIP != "10.0.0.5" || ok.UserAgent != "sync/1.0" || ok.RequestID != "req-1234567890abcdef" {
		t.Fatalf("client = %q %q %q", ok.ClientIP, ok.UserAgent, ok.RequestID)
	}
	if ok.Duration != 1500*time.Microsecond {
		t.Fatalf("duration = %v", ok.Duration)
	}
	if ok.Attrs["items_ok"] != 3 || ok.Attrs["items_failed"] != 1 {
		t.Fatalf("attrs = %v", ok.Attrs)
	}

	unauth := got[1]
	if unauth.Code != "unauthenticated" || unauth.Error != "missing api key" || !unauth.Principal.IsZero() || unauth.Attrs != nil {
		t.Fatalf("unauthenticated entry = %+v", unauth)
	}
	denied := got[2]
	if denied.Code != "permission_denied" || denied.Error != "no access" || !denied.Failed() {
		t.Fatalf("denied entry = %+v", denied)
	}
}

func TestMiddlewareAndInterceptorProduceOneEntry(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	opts := accesslog.CaptureOptions{
		TrustProxy: true,
		RequestID:  func(context.Context) string { return "" },
		Skip:       func(r *http.Request) bool { return r.URL.Path == "/healthz" },
	}
	server := echoServer(t, accesslog.Interceptor(l, opts), authInterceptor())
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	mux.HandleFunc("GET /files/{name}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("name") == "missing" {
			http.NotFound(w, r)
			return
		}
		accesslog.SetPrincipal(r.Context(), accesslog.Principal{Type: accesslog.PrincipalUser, ID: "7", Name: "alice"})
		_, _ = w.Write([]byte("data"))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// The response header stands in for a request ID middleware inside.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(accesslog.HeaderRequestID, "generated-request-id-1")
		mux.ServeHTTP(w, r)
	})
	handler := accesslog.Middleware(l, opts)(inner)

	call(t, handler, "deny", map[string]string{"X-Api-Key": "k", "X-Forwarded-For": "203.0.113.9, 10.0.0.1"}, "10.0.0.1:1")
	for _, path := range []string{"/files/a.txt", "/files/missing", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "[::1]:9000"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	closeLogger(t, l)

	got := sink.entries()
	if len(got) != 3 {
		t.Fatalf("entries = %+v", got)
	}
	rpc := got[0]
	if rpc.Kind != accesslog.KindRPC || rpc.Procedure != testProcedure || rpc.Code != "permission_denied" || rpc.Status != http.StatusForbidden {
		t.Fatalf("rpc entry = %+v", rpc)
	}
	if rpc.ClientIP != "203.0.113.9" || rpc.RequestID != "generated-request-id-1" || rpc.Principal.Name != "k" {
		t.Fatalf("rpc client = %+v", rpc)
	}
	file := got[1]
	if file.Kind != accesslog.KindHTTP || file.Method != http.MethodGet || file.Path != "/files/a.txt" || file.Status != 200 || file.Code != "" {
		t.Fatalf("http entry = %+v", file)
	}
	if file.ClientIP != "::1" || file.Principal.ID != "7" || file.Failed() {
		t.Fatalf("http client = %+v", file)
	}
	missing := got[2]
	if missing.Status != http.StatusNotFound || missing.Error != "Not Found" || !missing.Failed() {
		t.Fatalf("404 entry = %+v", missing)
	}
}

func TestCaptureSkipsAndNilLogger(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	opts := accesslog.CaptureOptions{SkipProcedure: func(p string) bool { return p == testProcedure }}
	server := echoServer(t, accesslog.Interceptor(l, opts), authInterceptor())
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	call(t, accesslog.Middleware(l, opts)(mux), "hi", map[string]string{"X-Api-Key": "k"}, "")
	call(t, mux, "hi", map[string]string{"X-Api-Key": "k"}, "")

	// Handlers may opt out of logging.
	skipped := accesslog.Middleware(l, accesslog.CaptureOptions{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accesslog.Skip(r.Context())
	}))
	skipped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/poll", nil))
	closeLogger(t, l)
	if got := sink.entries(); len(got) != 0 {
		t.Fatalf("skipped requests were logged: %+v", got)
	}

	// A nil Logger leaves handlers untouched and the context helpers are
	// no-ops.
	var disabled *accesslog.Logger
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accesslog.SetAttr(r.Context(), "k", 1) || accesslog.Active(r.Context()) {
			t.Error("context helpers active without a logger")
		}
	})
	accesslog.Middleware(disabled, accesslog.CaptureOptions{})(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestErrorIsTruncated(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}})
	server := connect.NewServer(accesslog.Interceptor(l, accesslog.CaptureOptions{}))
	long := strings.Repeat("错", accesslog.MaxErrorLength) // 3 bytes per rune
	server.Register(unary(func(context.Context, *wrapperspb.StringValue) (*emptypb.Empty, error) {
		return nil, connect.NewError(connect.CodeInternal, long)
	}))
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	call(t, mux, "x", nil, "")
	closeLogger(t, l)
	got := sink.entries()[0].Error
	if len(got) > accesslog.MaxErrorLength || len(got) < accesslog.MaxErrorLength-3 || !strings.HasPrefix(long, got) {
		t.Fatalf("error length = %d", len(got))
	}
}

func TestClientIP(t *testing.T) {
	h := http.Header{}
	h.Set("X-Forwarded-For", " 198.51.100.7 , 10.0.0.1")
	if got := accesslog.ClientIP(h, "10.0.0.1:80", true); got != "198.51.100.7" {
		t.Fatalf("trusted = %q", got)
	}
	if got := accesslog.ClientIP(h, "10.0.0.1:80", false); got != "10.0.0.1" {
		t.Fatalf("untrusted = %q", got)
	}
	if got := accesslog.ClientIP(http.Header{}, "pipe", true); got != "pipe" {
		t.Fatalf("no port = %q", got)
	}
}
