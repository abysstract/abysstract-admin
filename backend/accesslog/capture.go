package accesslog

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
)

// HeaderRequestID is the request ID header read when CaptureOptions.RequestID
// is nil.
const HeaderRequestID = "X-Request-Id"

// CaptureOptions configures Middleware and Interceptor.
type CaptureOptions struct {
	// TrustProxy takes the client IP from the left-most X-Forwarded-For
	// address. Enable it only behind a reverse proxy that sets the header;
	// otherwise the peer address is used so clients cannot forge it.
	TrustProxy bool
	// RequestID returns the request ID of a request context. nil reads the
	// X-Request-Id request header and, in Middleware, the response header
	// set by an inner handler.
	RequestID func(ctx context.Context) string
	// Skip excludes HTTP requests from Middleware, such as health probes.
	Skip func(r *http.Request) bool
	// SkipProcedure excludes Connect procedures from Interceptor.
	SkipProcedure func(procedure string) bool
	// Now returns the current time; nil selects time.Now. Tests use it.
	Now func() time.Time
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
// When l is nil the middleware returns next unchanged.
func Middleware(l *Logger, opts CaptureOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if l == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				ClientIP:  ClientIP(r.Header, r.RemoteAddr, opts.TrustProxy),
				UserAgent: truncate(r.UserAgent(), maxUserAgentLength),
			}}
			if opts.RequestID != nil {
				rec.entry.RequestID = opts.RequestID(r.Context())
			} else {
				rec.entry.RequestID = r.Header.Get(HeaderRequestID)
			}
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
// interceptors can call SetPrincipal.
//
// Behind Middleware it completes the request's entry with the procedure and
// Connect code. Used alone, as in a separate API server, it creates and logs
// the entry itself; Entry.Status is then 0.
func Interceptor(l *Logger, opts CaptureOptions) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		if l == nil {
			return next
		}
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
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
				rec = &record{entry: Entry{Time: start}}
				if info, ok := connect.CallInfoForServerContext(ctx); ok {
					header := info.RequestHeader()
					rec.entry.ClientIP = ClientIP(header, info.PeerAddr, opts.TrustProxy)
					rec.entry.UserAgent = truncate(header.Get("User-Agent"), maxUserAgentLength)
					rec.entry.RequestID = header.Get(HeaderRequestID)
				}
				if opts.RequestID != nil {
					if id := opts.RequestID(ctx); id != "" {
						rec.entry.RequestID = id
					}
				}
				ctx = withRecord(ctx, rec)
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

// HeaderGetter is satisfied by http.Header and *connect.Header.
type HeaderGetter interface {
	Get(key string) string
}

// ClientIP returns the client address of a request. With trustProxy it is
// the left-most X-Forwarded-For address when present; otherwise the peer
// address without its port.
func ClientIP(header HeaderGetter, peerAddr string, trustProxy bool) string {
	if trustProxy && header != nil {
		if xff := header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				xff = xff[:i]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return truncate(ip, 64)
			}
		}
	}
	if peerAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(peerAddr); err == nil {
		return host
	}
	return truncate(peerAddr, 64)
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
