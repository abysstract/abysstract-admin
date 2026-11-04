package observ

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/quick"

	"github.com/prometheus/client_golang/prometheus"
)

// Feature: open-source-readiness, Property 11: 请求 ID 传播
//
// 对任意入站请求：
//   - 若不含请求 ID，则生成长度 ≥16 字符的请求 ID 并在响应头回写
//   - 若已携带请求 ID（≥16 字符），则响应头回写的请求 ID 与入站值完全一致
//   - 若入站 ID 不足 16 字符，则视为不合法，生成新的 ID
func TestRequestIDPropagation_Property11(t *testing.T) {
	// 子测试 1：入站无 X-Request-Id → 响应头回写的 ID 长度 ≥ 16。
	t.Run("missing_header_generates_long_id", func(t *testing.T) {
		prop := func(method, path string) bool {
			if !validHTTPMethod(method) {
				return true // 跳过非法方法
			}
			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rid := RequestIDFromContext(r.Context())
				if rid == "" {
					t.Errorf("request ID missing from context")
					return
				}
				if len(rid) < minRequestIDLen {
					t.Errorf("generated ID len = %d, want ≥ %d", len(rid), minRequestIDLen)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(method, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get(HeaderRequestID)
			return len(got) >= minRequestIDLen
		}
		if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
			t.Fatalf("Property 11 (missing header) failed: %v", err)
		}
	})

	// 子测试 2：入站带合法 X-Request-Id（≥16 字符）→ 响应头回写值与入站完全一致。
	t.Run("inbound_header_is_echoed", func(t *testing.T) {
		prop := func(inboundID string) bool {
			if len(inboundID) < minRequestIDLen || !isPrintableASCII(inboundID) {
				return true
			}
			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rid := RequestIDFromContext(r.Context())
				if rid != inboundID {
					t.Errorf("context ID = %q, want %q", rid, inboundID)
				}
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderRequestID, inboundID)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get(HeaderRequestID)
			return got == inboundID
		}
		if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
			t.Fatalf("Property 11 (echo) failed: %v", err)
		}
	})

	// 子测试 3：入站 ID 长度 < 16 → 视为不合法，生成新的（不沿用）。
	t.Run("short_inbound_id_is_replaced", func(t *testing.T) {
		shortIDs := []string{"", "abc", "123456789012345"} // 长度 0..15
		for _, sid := range shortIDs {
			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if sid != "" {
				req.Header.Set(HeaderRequestID, sid)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get(HeaderRequestID)
			if len(got) < minRequestIDLen {
				t.Errorf("short inbound %q → response ID len = %d, want ≥ %d", sid, len(got), minRequestIDLen)
			}
			if got == sid {
				t.Errorf("short inbound %q should be replaced, but was echoed", sid)
			}
		}
	})
}

// TestLoggerIncludesRequestID 验证结构化日志包含 request_id/method/path/status 字段（需求 8.4）。
func TestLoggerIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// 中间件顺序：RequestID（注入 context） → Logger（请求结束时读取 context 中的 ID）。
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := RequestIDMiddleware(LoggerMiddleware(logger, inner))

	req := httptest.NewRequest(http.MethodPost, "/foo", nil)
	req.Header.Set(HeaderRequestID, "test-request-id-1234567890")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"request_id":"test-request-id-1234567890"`) {
		t.Errorf("log output missing request_id field:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, `"method":"POST"`) {
		t.Errorf("log output missing method field:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, `"path":"/foo"`) {
		t.Errorf("log output missing path field:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("log output missing status field:\n%s", logOutput)
	}
}

// TestMetricsRecordRequest 验证 metrics 中间件捕获状态码。
func TestMetricsRecordRequest(t *testing.T) {
	// 用独立 registry 避免与全局 DefaultRegisterer 冲突（重复注册同名 metric 会 panic）。
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	handler := m.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPut, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// metrics 内部状态不便直接断言（依赖 prometheus registry），
	// 这里仅验证中间件不破坏 handler 行为。
	if rec.Result().StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}

	// 验证指标确实被采集：通过 registry gather 检查 counter > 0。
	fam, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := false
	for _, mf := range fam {
		if mf.GetName() == "http_requests_total" {
			for _, m := range mf.GetMetric() {
				if m.GetCounter().GetValue() > 0 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("http_requests_total not incremented")
	}
}

// ---------- 辅助函数 ----------

func validHTTPMethod(s string) bool {
	switch s {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodHead, http.MethodOptions, http.MethodPatch:
		return true
	}
	return false
}

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

var _ = context.Background // 占位，保留 context import 以备后续扩展。
