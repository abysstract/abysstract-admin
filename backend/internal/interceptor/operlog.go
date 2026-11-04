package interceptor

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// OperLogEntry 一条操作日志。
type OperLogEntry struct {
	Title     string
	Method    string
	OperName  string
	OperID    int64
	OperIP    string
	OperParam string
	Status    int16 // 1 成功 2 失败
	ErrorMsg  string
	CostMs    int64
}

// maxParamLen 参数摘要写入上限（与 sys_oper_log.oper_param 列 VARCHAR(2000) 对齐，留余量）。
const maxParamLen = 1900

// NewOperLogInterceptor 记录写操作（跳过读操作、AuthService、LogService）。
// 须注册在 Auth/Permission 之后（最内层），以读取 Claims 并仅记录已通过鉴权的调用。
// trustProxy 为 true 时才信任 X-Forwarded-For，否则使用直连对端地址，避免 IP 伪造。
func NewOperLogInterceptor(record func(context.Context, OperLogEntry), trustProxy bool) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			proc := spec.Procedure
			if !shouldAudit(proc) {
				return next(ctx, spec, stream)
			}
			info, _ := connect.CallInfoForServerContext(ctx)
			start := time.Now()
			audit := &auditStream{ServerStream: stream}
			err := next(ctx, spec, audit)
			if !audit.received {
				return err
			}

			entry := OperLogEntry{
				Title:     titleOf(proc),
				Method:    proc,
				OperIP:    clientIP(info, trustProxy),
				OperParam: audit.param,
				Status:    1,
				CostMs:    time.Since(start).Milliseconds(),
			}
			if c, ok := ClaimsFromContext(ctx); ok {
				entry.OperName = c.Username
				entry.OperID = c.UserID
			}
			if err != nil {
				entry.Status = 2
				entry.ErrorMsg = err.Error()
			}
			record(ctx, entry)
			return err
		}
	}
}

// auditStream captures the decoded request after authentication and permission checks.
type auditStream struct {
	connect.ServerStream
	param    string
	received bool
}

func (s *auditStream) Receive(msg any) error {
	if err := s.ServerStream.Receive(msg); err != nil {
		return err
	}
	s.received = true
	s.param = paramSummary(msg)
	return nil
}

// shouldAudit 仅审计写操作。
func shouldAudit(procedure string) bool {
	i := strings.LastIndex(procedure, "/")
	if i < 0 {
		return false
	}
	service := procedure[:i]
	method := procedure[i+1:]
	if strings.Contains(service, "AuthService") || strings.Contains(service, "LogService") {
		return false
	}
	if strings.HasPrefix(method, "List") || strings.HasPrefix(method, "Get") {
		return false
	}
	return true
}

func titleOf(procedure string) string {
	i := strings.LastIndex(procedure, "/")
	if i < 0 {
		return procedure
	}
	method := procedure[i+1:]
	rest := procedure[:i]
	service := rest[strings.LastIndex(rest, ".")+1:]
	return service + "." + method
}

// clientIP 解析客户端 IP。仅在信任反向代理时采用 X-Forwarded-For 最左地址，
// 否则使用直连对端地址（去掉端口），避免外部伪造 XFF 头污染审计日志。
func clientIP(info *connect.CallInfo, trustProxy bool) string {
	if trustProxy {
		if xff := info.RequestHeader().Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx >= 0 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}
	if addr := info.PeerAddr; addr != "" {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			return host
		}
		return addr
	}
	return ""
}

// paramSummary 把请求消息序列化为 JSON 摘要，脱敏敏感字段并截断超长内容。
func paramSummary(value any) string {
	msg, ok := value.(proto.Message)
	if !ok {
		return ""
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return truncateParam(string(raw))
	}
	redactSensitive(m)
	out, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return truncateParam(string(out))
}

// redactSensitive 递归脱敏 map 中的敏感字段：
//  1. 字段名本身敏感（password/secret/token/apiKey 等）→ 直接脱敏其值；
//  2. 形如 <x>Key + <x>Value 的「键/值」对（如参数配置 configKey/configValue）：
//     当键名的取值看起来是敏感项（如 jwt.secret、openai.api_key）时，脱敏对应的值。
func redactSensitive(v any) {
	switch t := v.(type) {
	case map[string]any:
		redactKeyValuePairs(t)
		for k, val := range t {
			if t[k] == redacted {
				continue
			}
			if isSensitiveName(k) {
				t[k] = redacted
				continue
			}
			redactSensitive(val)
		}
	case []any:
		for _, e := range t {
			redactSensitive(e)
		}
	}
}

const redacted = "***"

// redactKeyValuePairs 处理「键名字段 + 值字段」成对出现的情况：
// 约定 <prefix>Key 搭配 <prefix>Value（含纯 "key"/"value"）。
// 若键名字段的字符串取值是敏感项，则脱敏其搭配的值字段。
func redactKeyValuePairs(m map[string]any) {
	for k, val := range m {
		keyName, ok := val.(string)
		if !ok {
			continue
		}
		valueField := ""
		switch {
		case strings.EqualFold(k, "key"):
			valueField = siblingField(m, "value")
		case strings.HasSuffix(k, "Key"):
			valueField = siblingField(m, strings.TrimSuffix(k, "Key")+"Value")
		case strings.HasSuffix(k, "_key"):
			valueField = siblingField(m, strings.TrimSuffix(k, "_key")+"_value")
		}
		if valueField != "" && isSensitiveName(keyName) {
			m[valueField] = redacted
		}
	}
}

// siblingField 在 map 中按名称（不区分大小写）查找字段，返回实际存在的键名。
func siblingField(m map[string]any, name string) string {
	if _, ok := m[name]; ok {
		return name
	}
	for k := range m {
		if strings.EqualFold(k, name) {
			return k
		}
	}
	return ""
}

// isSensitiveName 判断字段名/键名是否暗示敏感信息。
func isSensitiveName(s string) bool {
	lk := strings.ToLower(s)
	for _, kw := range []string{
		"password", "passwd", "secret", "token",
		"apikey", "api_key", "credential", "private_key", "privatekey",
		"cookie", "auth",
	} {
		if strings.Contains(lk, kw) {
			return true
		}
	}
	return false
}

func truncateParam(s string) string {
	if len(s) <= maxParamLen {
		return s
	}
	return s[:maxParamLen] + "...(truncated)"
}
