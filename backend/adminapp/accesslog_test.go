package adminapp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
)

func TestAccessLogConfigPurge(t *testing.T) {
	cfg := adminconfig.Config{AccessLogRetentionDays: 30}
	if got := AccessLogConfig(cfg, nil, nil).Retention; got != 30*24*time.Hour {
		t.Fatalf("retention = %s", got)
	}
	// A process that only writes hands the db sink no retention, so it
	// never starts the purge loop.
	cfg.AccessLogPurgeDisabled = true
	if got := AccessLogConfig(cfg, nil, nil).Retention; got != 0 {
		t.Fatalf("retention with purge disabled = %s", got)
	}
}

func TestAccessLogCaptureSkipsProbes(t *testing.T) {
	capture := AccessLogCapture(adminconfig.Config{TrustProxy: true})
	if !capture.TrustProxy || capture.RequestID == nil || capture.AssignRequestID {
		t.Fatalf("capture = %+v", capture)
	}
	for path, want := range map[string]bool{"/healthz": true, "/readyz": true, "/metrics": true, "/version": true, "/abysstract.admin.v1.AuthService/Login": false} {
		if got := capture.Skip(httptest.NewRequest(http.MethodPost, path, nil)); got != want {
			t.Errorf("Skip(%s) = %v", path, got)
		}
	}
	if !capture.Skip(httptest.NewRequest(http.MethodOptions, "/abysstract.admin.v1.AuthService/Login", nil)) {
		t.Error("CORS preflight not skipped")
	}
}
