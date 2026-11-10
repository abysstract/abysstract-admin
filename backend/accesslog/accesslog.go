// Package accesslog records one entry per request and hands batches of entries
// to replaceable sinks.
//
// Requests never wait for a sink: Middleware and Interceptor capture an Entry
// and Logger.Log puts it on a bounded in-memory queue. A background worker
// writes batches to every configured Sink. When the queue is full the entry
// is dropped and counted in Stats, so access logging can never block or fail
// a request. Use an audit log, not this package, for records that must not be
// lost.
//
// Sinks are selected by name (see Register and Open). The built-in sinks are
// accesslog/stdoutsink ("stdout", one JSON line per request) and
// accesslog/dbsink ("db", the sys_access_log table, which also implements
// Reader for the admin page).
package accesslog

import (
	"context"
	"time"
)

// Entry kinds.
const (
	KindRPC  = "rpc"
	KindHTTP = "http"
)

// Well-known principal types. Applications may use their own values.
const (
	PrincipalUser          = "user"
	PrincipalAPICredential = "api_credential"
)

// Principal identifies the caller of a request.
type Principal struct {
	// Type is the kind of caller, such as PrincipalUser or
	// PrincipalAPICredential. Empty means anonymous.
	Type string
	// ID is the stable identifier of the caller, for example a user ID or
	// an API key ID.
	ID string
	// Name is a human-readable caller name, such as a username.
	Name string
}

// IsZero reports whether no principal has been set.
func (p Principal) IsZero() bool { return p == Principal{} }

// Entry is one request.
type Entry struct {
	// ID is assigned by sinks that store entries; it is empty when an entry
	// is captured and set on entries returned by a Reader.
	ID string
	// Time is when the request started.
	Time time.Time
	// RequestID correlates the entry with application logs and the
	// X-Request-Id response header.
	RequestID string
	// Kind is KindRPC for Connect calls and KindHTTP for other requests.
	Kind string
	// Service and Procedure identify a Connect call, for example
	// "abysstract.admin.v1.UserService" and
	// "/abysstract.admin.v1.UserService/ListUsers".
	Service   string
	Procedure string
	// Method and Path are the HTTP request line.
	Method string
	Path   string

	Principal Principal
	// ClientIP is the peer address, or the left-most X-Forwarded-For address
	// when the proxy is trusted.
	ClientIP  string
	UserAgent string

	// Code is the Connect code ("ok", "not_found", ...) of a Connect call and
	// empty for other requests.
	Code string
	// Status is the HTTP status code, or 0 when it is unknown (an
	// Interceptor used without Middleware).
	Status int
	// Error is the error message of a failed request, truncated to
	// MaxErrorLength bytes.
	Error    string
	Duration time.Duration

	// Attrs holds business fields added with SetAttr and AddAttrs, such as
	// item counts of a batch call. Values must be JSON-encodable. Sinks
	// must treat the map as read-only.
	Attrs map[string]any
}

// Failed reports whether the request failed: a Connect code other than ok,
// or an HTTP status of 400 or above for other requests.
func (e Entry) Failed() bool {
	if e.Code != "" {
		return e.Code != "ok"
	}
	return e.Status >= 400
}

// MaxErrorLength bounds Entry.Error.
const MaxErrorLength = 1000

// Sink receives batches of entries from the Logger worker.
//
// Write is called from a single goroutine per sink, never concurrently with
// itself or Close. The batch slice and the entries' Attrs maps are shared
// between sinks and must not be modified or retained after Write returns.
// A sink that fails returns an error; the Logger counts it and continues with
// the next batch, without retrying.
type Sink interface {
	Write(ctx context.Context, entries []Entry) error
	Close(ctx context.Context) error
}

// Namer is implemented by sinks that report a name in Stats. Sinks without
// it are named by their position.
type Namer interface {
	Name() string
}

// Result filters entries by outcome.
type Result string

const (
	ResultAny     Result = ""
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

// Query selects entries from a Reader. Zero fields do not filter.
type Query struct {
	// From and To bound Entry.Time: From inclusive, To exclusive.
	From, To time.Time
	// Principal matches Principal.ID or Principal.Name exactly.
	Principal string
	// PrincipalType matches Principal.Type exactly.
	PrincipalType string
	// Procedure matches Procedure or Path: a value starting with "/" is a
	// prefix, any other value a substring.
	Procedure string
	Result    Result
	// Code matches Entry.Code exactly.
	Code      string
	RequestID string
	// Offset and Limit page the results, newest first. Limit 0 selects the
	// reader's default page size.
	Offset, Limit int
}

// Page is one page of query results.
type Page struct {
	Entries []Entry
	Total   int64
}

// Reader is implemented by sinks that can query stored entries. The admin
// "请求日志" page needs a configured sink that implements it.
type Reader interface {
	Query(ctx context.Context, q Query) (Page, error)
}
