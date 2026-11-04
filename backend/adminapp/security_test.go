package adminapp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/identity"
)

func TestIdentityFromWithClaims(t *testing.T) {
	ctx := context.Background()
	if _, ok := identity.FromContext(ctx); ok {
		t.Fatal("anonymous")
	}
}

func TestJSONErrorShape(t *testing.T) {
	var buf bytes.Buffer
	writeHTTPError(&statusCapturingWriter{buf: &buf, header: http.Header{}}, 401, "unauthenticated", "未认证")
	var body httpErrorBody
	if err := json.Unmarshal(buf.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "unauthenticated" || body.Message != "未认证" {
		t.Fatalf("%+v", body)
	}
}

type statusCapturingWriter struct {
	header http.Header
	buf    *bytes.Buffer
	status int
}

func (w *statusCapturingWriter) Header() http.Header { return w.header }

func (w *statusCapturingWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *statusCapturingWriter) WriteHeader(status int) { w.status = status }
