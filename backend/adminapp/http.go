package adminapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/module"
)

type httpSecurity struct {
	manager     *jwt.Manager
	verLoader   interceptor.TokenVersionLoader
	resetLoader interceptor.PwdResetRequiredLoader
	permLoader  interceptor.PermLoader
	trustProxy  bool
	record      func(interceptor.OperLogEntry)
}

type httpErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func wrapHTTPRoute(route preparedRoute, handler http.Handler, sec httpSecurity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if route.Route.Access != module.AccessPublic {
			claims, err := interceptor.VerifyBearerToken(ctx, r.Header.Get("Authorization"), sec.manager, sec.verLoader)
			if err != nil {
				writeHTTPError(w, http.StatusUnauthorized, "unauthenticated", "未认证")
				return
			}
			if err := interceptor.EnforcePwdReset(ctx, route.Route.Name, claims, sec.resetLoader); err != nil {
				if errors.Is(err, interceptor.ErrPwdResetRequired) {
					writeHTTPError(w, http.StatusForbidden, "permission_denied", "无权限")
					return
				}
				writeHTTPError(w, http.StatusInternalServerError, "internal", "内部错误")
				return
			}
			ctx = interceptor.WithClaims(ctx, claims)
			if route.Route.Access == module.AccessPermission {
				perms, err := sec.permLoader(ctx, claims.UserID)
				if err != nil {
					writeHTTPError(w, http.StatusInternalServerError, "internal", "内部错误")
					return
				}
				if !interceptor.HasPerm(perms, route.Route.Permission) {
					writeHTTPError(w, http.StatusForbidden, "permission_denied", "无权限")
					return
				}
			}
		}
		body, err := readLimitedBody(r, route.MaxBytes)
		if err != nil {
			writeHTTPError(w, http.StatusRequestEntityTooLarge, "request_too_large", "请求体过大")
			return
		}
		r = r.WithContext(ctx)
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		handler.ServeHTTP(rec, r)
		if sec.record != nil && shouldAuditHTTP(route.Route.Method) {
			entry := interceptor.OperLogEntry{
				Title:     route.Route.Name,
				Method:    route.Route.Method + " " + route.Route.Path,
				OperIP:    httpClientIP(r, sec.trustProxy),
				OperParam: httpBodySummary(r.Header.Get("Content-Type"), body),
				Status:    1,
			}
			if claims, ok := interceptor.ClaimsFromContext(ctx); ok {
				entry.OperName = claims.Username
				entry.OperID = claims.UserID
			}
			if rec.status >= 400 {
				entry.Status = 2
			}
			sec.record(entry)
		}
	})
}

func readLimitedBody(r *http.Request, limit int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, limit)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeHTTPError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(httpErrorBody{Code: code, Message: message})
}

func shouldAuditHTTP(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func httpClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx >= 0 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}
	host, _, ok := strings.Cut(r.RemoteAddr, ":")
	if ok {
		return host
	}
	return r.RemoteAddr
}

func httpBodySummary(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if strings.Contains(strings.ToLower(contentType), "json") && utf8.Valid(body) {
		var payload any
		if err := json.Unmarshal(body, &payload); err == nil {
			redactJSON(payload)
			raw, err := json.Marshal(payload)
			if err == nil {
				return truncate(string(raw), 1900)
			}
		}
	}
	return truncate(contentType+" size="+strconv.Itoa(len(body)), 1900)
}

func redactJSON(v any) {
	switch item := v.(type) {
	case map[string]any:
		for key, value := range item {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") {
				item[key] = "[redacted]"
				continue
			}
			redactJSON(value)
		}
	case []any:
		for _, value := range item {
			redactJSON(value)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
