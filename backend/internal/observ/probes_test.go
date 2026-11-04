package observ

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHealthz 验证 /healthz 立即返回 200（需求 8.1：<1s，不依赖 DB）。
func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(HealthzHandler())
	defer srv.Close()

	start := time.Now()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("/healthz took %v, want < 1s", elapsed)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "alive" {
		t.Errorf("body.status = %q, want %q", body["status"], "alive")
	}
}

// TestReadyz 验证 /readyz 在 DB 可用时返回 ready，不可用时返回 not_ready（需求 8.2/8.3）。
func TestReadyz(t *testing.T) {
	t.Run("db_ok_returns_ready", func(t *testing.T) {
		srv := httptest.NewServer(ReadyzHandler(stubPinger{err: nil}))
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body["status"] != "ready" {
			t.Errorf("body = %v, want ready", body)
		}
	})

	t.Run("db_fail_returns_not_ready_503", func(t *testing.T) {
		srv := httptest.NewServer(ReadyzHandler(stubPinger{err: errors.New("db down")}))
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
		}
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body["status"] != "not_ready" {
			t.Errorf("body = %v, want not_ready", body)
		}
	})

	t.Run("nil_pinger_returns_not_ready", func(t *testing.T) {
		srv := httptest.NewServer(ReadyzHandler(nil))
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
		}
	})
}

// TestVersionHandler 验证 /version 返回构建信息（需求 8.13）。
func TestVersionHandler(t *testing.T) {
	srv := httptest.NewServer(VersionHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 默认值（未通过 ldflags 注入）应为 dev/unknown。
	if body["version"] == "" {
		t.Error("version field empty")
	}
}

// TestMetricsHandler 验证 /metrics 输出 Prometheus 格式（需求 8.7）。
func TestMetricsHandler(t *testing.T) {
	// 先发一个请求触发 counter。
	m := NewMetrics()
	wrapped := m.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req1 := httptest.NewRequest(http.MethodGet, "/x", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req1)

	// 然后查询 metrics endpoint。
	srv := httptest.NewServer(MetricsHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// body 应包含 Prometheus 文本格式的指标行（# HELP / # TYPE）。
	body := readBody(t, resp)
	if !strings.Contains(body, "http_requests_total") {
		t.Errorf("metrics body missing http_requests_total:\n%s", body)
	}
}

// TestGracefulShutdown 集成测试：在途请求 + Shutdown → 请求正常完成后退出。
// 覆盖需求 8.8（优雅停机）。
func TestGracefulShutdown(t *testing.T) {
	// 启动一个故意慢 200ms 的 handler，模拟在途请求。
	inFlight := make(chan struct{})
	handlerStarted := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		close(inFlight) // 请求已完成
	})

	srv := &http.Server{Addr: "127.0.0.1:0", Handler: mux}
	addr := startOnFreePort(t, srv)

	// 在后台发请求（在途）。
	var (
		respStatus int
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			t.Errorf("in-flight GET: %v", err)
			return
		}
		respStatus = resp.StatusCode
		_ = resp.Body.Close()
	}()

	// 等 handler 真的开始处理（确认在途），再触发 shutdown。
	<-handlerStarted
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	wg.Wait()
	if respStatus != http.StatusOK {
		t.Errorf("in-flight request status = %d, want 200 (should complete)", respStatus)
	}
	<-inFlight
}

// TestShutdownTimeoutForcesExit 集成测试：在途请求超过超时 → Close 强退。
// 覆盖需求 8.9（超时强退）。
func TestShutdownTimeoutForcesExit(t *testing.T) {
	handlerStarted := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/forever", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		time.Sleep(5 * time.Second) // 远超 shutdown timeout
	})

	srv := &http.Server{Addr: "127.0.0.1:0", Handler: mux}
	addr := startOnFreePort(t, srv)

	// 在途请求 + Shutdown 立即超时 → 应返回 context deadline exceeded。
	go func() {
		_, _ = http.Get("http://" + addr + "/forever")
	}()
	<-handlerStarted

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := srv.Shutdown(shutdownCtx)
	if err == nil {
		t.Error("expected Shutdown to time out, got nil")
	}
	// 强制关闭，让慢请求连接断开。
	_ = srv.Close()
}

// ---------- 辅助函数 ----------

type stubPinger struct{ err error }

func (s stubPinger) PingContext(ctx context.Context) error { return s.err }

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}

// startOnFreePort 启动 srv（地址 :0 让 OS 分配端口），返回实际监听地址。
// 失败时 t.Fatal。调用方负责 srv.Close()。
func startOnFreePort(t *testing.T, srv *http.Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String()
}
