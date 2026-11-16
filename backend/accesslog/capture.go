package accesslog

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
)

// HeaderRequestID is the request ID header read when CaptureOptions.RequestID
// is nil.
const HeaderRequestID = "X-Request-Id"

// CaptureOptions configures Middleware and Interceptor.
type CaptureOptions struct {
	// TrustProxy and TrustedProxies select the client IP; see ProxyPolicy.
	// Leave both unset unless a reverse proxy sets X-Forwarded-For, so
	// clients cannot forge the address.
	TrustProxy     bool
	TrustedProxies []netip.Prefix
	// AssignRequestID gives every request a request ID, for servers without
	// a request ID middleware of their own: a valid client X-Request-Id
	// (see ValidRequestID) is kept, otherwise NewRequestID generates one.
	// The ID is set as the X-Request-Id response header, stored in the
	// context for RequestIDFromContext and recorded in Entry.RequestID.
	// Middleware assigns it to every request, skipped ones included;
	// Interceptor assigns it when it runs without Middleware.
	AssignRequestID bool
	// RequestID returns the request ID of a request context. nil uses
	// RequestIDFromContext, then the X-Request-Id request header and, in
	// Middleware, the response header set by an inner handler.
	RequestID func(ctx context.Context) string
	// Skip excludes HTTP requests from Middleware; nil logs every request.
	// DefaultSkip and SkipPaths skip CORS preflight and health probes.
	Skip func(r *http.Request) bool
	// SkipProcedure excludes Connect procedures from Interceptor.
	SkipProcedure func(procedure string) bool
	// Now returns the current time; nil selects time.Now. Tests use it.
	Now func() time.Time
}

func (o CaptureOptions) proxyPolicy() ProxyPolicy {
	return ProxyPolicy{Trust: o.TrustProxy, Proxies: o.TrustedProxies}
}

func (o CaptureOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Middleware captures one entry per HTTP request and logs it when the
// handler returns. Connect calls served by the wrapped handler are completed
// by Interceptor, which adds the procedure and Connect code; without the
// Interceptor they are logged as plain HTTP requests.
//
// When l is nil nothing is logged; the middleware then only assigns request
// IDs when opts.AssignRequestID is set and otherwise returns next unchanged.
func Middleware(l *Logger, opts CaptureOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if l == nil {
			if !opts.AssignRequestID {
				return next
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, opts.assignHTTP(w, r))
			})
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = opts.assignHTTP(w, r)
			if (opts.Skip != nil && opts.Skip(r)) || Active(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			start := opts.now()
			rec := &record{entry: Entry{
				Time:      start,
				Kind:      KindHTTP,
				Method:    r.Method,
				Path:      r.URL.Path,
				ClientIP:  opts.proxyPolicy().ClientIP(r.Header, r.RemoteAddr),
				UserAgent: truncate(r.UserAgent(), maxUserAgentLength),
			}}
			rec.entry.RequestID = opts.requestID(r.Context(), r.Header)
			sw := &statusWriter{ResponseWriter: w}
			defer func() {
				status := sw.status
				if status == 0 {
					status = http.StatusOK
				}
				rec.mu.Lock()
				rec.entry.Status = status
				rec.entry.Duration = opts.now().Sub(start)
				if rec.entry.RequestID == "" {
					rec.entry.RequestID = w.Header().Get(HeaderRequestID)
				}
				if !rec.rpc && rec.entry.Error == "" && status >= 400 {
					rec.entry.Error = http.StatusText(status)
				}
				rec.mu.Unlock()
				if e, ok := rec.snapshot(); ok {
					l.Log(e)
				}
			}()
			next.ServeHTTP(sw, r.WithContext(withRecord(r.Context(), rec)))
		})
	}
}

// Interceptor captures Connect calls. Register it first (outermost) on the
// connect.Server so authentication failures are recorded and auth
// interceptors can call SetPrincipal. With a nil l it logs nothing and only
// assigns request IDs when opts.AssignRequestID is set.
//
// Behind Middleware it completes the request's entry with the procedure and
// Connect code. Used alone, as in a separate API server, it creates and logs
// the entry itself; Entry.Status is then 0.
func Interceptor(l *Logger, opts CaptureOptions) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		if l == nil {
			if !opts.AssignRequestID {
				return next
			}
			return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				return next(opts.assignRPC(ctx), spec, stream)
			}
		}
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			ctx = opts.assignRPC(ctx)
			if opts.SkipProcedure != nil && opts.SkipProcedure(spec.Procedure) {
				if rec := recordFrom(ctx); rec != nil {
					rec.mu.Lock()
					rec.skip = true
					rec.mu.Unlock()
				}
				return next(ctx, spec, stream)
			}
			rec := recordFrom(ctx)
			own := rec == nil
			var start time.Time
			if own {
				start = opts.now()
				ctx, rec = opts.newRPCRecord(ctx, start)
			}
			rec.mu.Lock()
			rec.rpc = true
			rec.entry.Kind = KindRPC
			rec.entry.Procedure = spec.Procedure
			rec.entry.Service = serviceOf(spec.Procedure)
			if own {
				rec.entry.Method = http.MethodPost
				rec.entry.Path = spec.Procedure
			}
			rec.mu.Unlock()

			err := next(ctx, spec, stream)

			rec.mu.Lock()
			rec.entry.Code = connect.CodeOf(err).String()
			if err == nil {
				rec.entry.Code = "ok"
			} else {
				rec.entry.Error = truncate(errorMessage(err), MaxErrorLength)
			}
			if own {
				rec.entry.Duration = opts.now().Sub(start)
			}
			rec.mu.Unlock()
			if own {
				if e, ok := rec.snapshot(); ok {
					l.Log(e)
				}
			}
			return err
		}
	}
}

const maxUserAgentLength = 255

// requestID returns the request ID of an HTTP request or Connect call.
func (o CaptureOptions) requestID(ctx context.Context, header HeaderGetter) string {
	if o.RequestID != nil {
		if id := o.RequestID(ctx); id != "" {
			return id
		}
	}
	if id := RequestIDFromContext(ctx); id != "" {
		return id
	}
	if header == nil {
		return ""
	}
	return truncate(header.Get(HeaderRequestID), MaxRequestIDLength)
}

// assignHTTP gives r a request ID when AssignRequestID is set and r has none.
func (o CaptureOptions) assignHTTP(w http.ResponseWriter, r *http.Request) *http.Request {
	if !o.AssignRequestID || RequestIDFromContext(r.Context()) != "" {
		return r
	}
	id := resolveRequestID(r.Header.Get(HeaderRequestID))
	w.Header().Set(HeaderRequestID, id)
	return r.WithContext(WithRequestID(r.Context(), id))
}

// assignRPC gives a Connect call a request ID when AssignRequestID is set and
// the context has none, as for a call served without Middleware.
func (o CaptureOptions) assignRPC(ctx context.Context) context.Context {
	if !o.AssignRequestID || RequestIDFromContext(ctx) != "" {
		return ctx
	}
	info, ok := connect.CallInfoForServerContext(ctx)
	if !ok {
		return ctx
	}
	id := resolveRequestID(info.RequestHeader().Get(HeaderRequestID))
	info.ResponseHeader().Set(HeaderRequestID, id)
	return WithRequestID(ctx, id)
}

// newRPCRecord starts the entry of a Connect call served without Middleware.
func (o CaptureOptions) newRPCRecord(ctx context.Context, start time.Time) (context.Context, *record) {
	rec := &record{entry: Entry{Time: start}}
	var header HeaderGetter
	if info, ok := connect.CallInfoForServerContext(ctx); ok {
		reqHeader := info.RequestHeader()
		header = reqHeader
		rec.entry.ClientIP = o.proxyPolicy().ClientIP(reqHeader, info.PeerAddr)
		rec.entry.UserAgent = truncate(reqHeader.Get("User-Agent"), maxUserAgentLength)
	}
	rec.entry.RequestID = o.requestID(ctx, header)
	return withRecord(ctx, rec), rec
}

func errorMessage(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		if msg := ce.Message(); msg != "" {
			return msg
		}
		return ce.Code().String()
	}
	return err.Error()
}

// serviceOf returns "pkg.Service" for "/pkg.Service/Method".
func serviceOf(procedure string) string {
	p := strings.TrimPrefix(procedure, "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// statusWriter records the response status and keeps the optional
// interfaces of the wrapped writer reachable through Unwrap.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
