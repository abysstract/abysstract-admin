// Package stdoutsink registers the "stdout" access log sink: one JSON object
// per line, ready for log shippers such as Filebeat, Fluent Bit or Promtail.
//
// Field names are stable:
//
//	type           always "access_log", to tell entries from application logs
//	time           RFC 3339 UTC time with nanoseconds when the request started
//	request_id     X-Request-Id of the request
//	kind           "rpc" or "http"
//	service        Connect service, such as "abysstract.admin.v1.UserService"
//	procedure      Connect procedure, such as "/abysstract.admin.v1.UserService/ListUsers"
//	method, path   HTTP request line
//	principal_type "user", "api_credential", ... or "" when anonymous
//	principal_id   caller ID
//	principal_name caller name
//	client_ip      client address
//	user_agent     User-Agent header
//	code           Connect code ("ok", "not_found", ...) or "" for plain HTTP
//	status         HTTP status, 0 when unknown
//	success        false for a non-ok code or an HTTP status of 400 or above
//	error          error message, omitted when empty
//	duration_ms    duration in milliseconds, with microsecond precision
//	attrs          business fields, omitted when empty
package stdoutsink

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

// Name is the registered sink name.
const Name = "stdout"

func init() {
	accesslog.Register(Name, func(_ context.Context, cfg accesslog.Config) (accesslog.Sink, error) {
		return New(cfg.Output), nil
	})
}

// Line is the JSON object written for one entry.
type Line struct {
	Type          string         `json:"type"`
	Time          string         `json:"time"`
	RequestID     string         `json:"request_id"`
	Kind          string         `json:"kind"`
	Service       string         `json:"service"`
	Procedure     string         `json:"procedure"`
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	PrincipalType string         `json:"principal_type"`
	PrincipalID   string         `json:"principal_id"`
	PrincipalName string         `json:"principal_name"`
	ClientIP      string         `json:"client_ip"`
	UserAgent     string         `json:"user_agent"`
	Code          string         `json:"code"`
	Status        int            `json:"status"`
	Success       bool           `json:"success"`
	Error         string         `json:"error,omitempty"`
	DurationMS    float64        `json:"duration_ms"`
	Attrs         map[string]any `json:"attrs,omitempty"`
}

// LineOf converts an entry to its JSON line representation.
func LineOf(e accesslog.Entry) Line {
	return Line{
		Type:          "access_log",
		Time:          e.Time.UTC().Format(time.RFC3339Nano),
		RequestID:     e.RequestID,
		Kind:          e.Kind,
		Service:       e.Service,
		Procedure:     e.Procedure,
		Method:        e.Method,
		Path:          e.Path,
		PrincipalType: e.Principal.Type,
		PrincipalID:   e.Principal.ID,
		PrincipalName: e.Principal.Name,
		ClientIP:      e.ClientIP,
		UserAgent:     e.UserAgent,
		Code:          e.Code,
		Status:        e.Status,
		Success:       !e.Failed(),
		Error:         e.Error,
		DurationMS:    math.Round(float64(e.Duration)/float64(time.Microsecond)) / 1000,
		Attrs:         e.Attrs,
	}
}

// Sink writes JSON lines to a writer.
type Sink struct {
	mu  sync.Mutex
	out io.Writer
}

// New returns a sink writing to out; nil selects os.Stdout. The writer is
// not closed by Close.
func New(out io.Writer) *Sink {
	if out == nil {
		out = os.Stdout
	}
	return &Sink{out: out}
}

// Name implements accesslog.Namer.
func (s *Sink) Name() string { return Name }

// Write writes one line per entry, each with a single Write call so lines
// are not interleaved with other writers of the same stream.
func (s *Sink) Write(_ context.Context, entries []accesslog.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		line := LineOf(e)
		data, err := json.Marshal(line)
		if err != nil {
			// Attrs holds a value JSON cannot encode; keep the entry.
			line.Attrs = map[string]any{"_encode_error": err.Error()}
			if data, err = json.Marshal(line); err != nil {
				return err
			}
		}
		data = append(data, '\n')
		if _, err := s.out.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// Close implements accesslog.Sink; it does not close the writer.
func (s *Sink) Close(context.Context) error { return nil }
