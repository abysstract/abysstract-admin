package accesslog_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

// memSink records batches. When gate is non-nil, Write blocks until it is
// closed.
type memSink struct {
	name string
	gate chan struct{}
	err  error

	mu      sync.Mutex
	batches [][]accesslog.Entry
	closed  bool
}

func (s *memSink) Name() string { return s.name }

func (s *memSink) Write(ctx context.Context, entries []accesslog.Entry) error {
	if s.gate != nil {
		select {
		case <-s.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batches = append(s.batches, append([]accesslog.Entry(nil), entries...))
	return nil
}

func (s *memSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *memSink) entries() []accesslog.Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []accesslog.Entry
	for _, b := range s.batches {
		out = append(out, b...)
	}
	return out
}

func (s *memSink) batchSizes() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int
	for _, b := range s.batches {
		out = append(out, len(b))
	}
	return out
}

func entry(i int) accesslog.Entry {
	return accesslog.Entry{RequestID: fmt.Sprintf("r%d", i), Kind: accesslog.KindRPC, Code: "ok"}
}

func closeLogger(t *testing.T, l *accesslog.Logger) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := l.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestLoggerBatchesBySize(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}, BatchSize: 3, FlushInterval: time.Hour})
	for i := range 7 {
		if !l.Log(entry(i)) {
			t.Fatalf("entry %d dropped", i)
		}
	}
	waitFor(t, func() bool { return len(sink.batchSizes()) == 2 })
	if got := sink.batchSizes(); got[0] != 3 || got[1] != 3 {
		t.Fatalf("batches before close = %v, want [3 3]", got)
	}
	closeLogger(t, l)
	if got := sink.batchSizes(); len(got) != 3 || got[2] != 1 {
		t.Fatalf("batches after close = %v, want [3 3 1]", got)
	}
	for i, e := range sink.entries() {
		if e.RequestID != fmt.Sprintf("r%d", i) {
			t.Fatalf("entry %d = %q, order not kept", i, e.RequestID)
		}
	}
}

func TestLoggerFlushesOnInterval(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}, BatchSize: 100, FlushInterval: 20 * time.Millisecond})
	defer closeLogger(t, l)
	l.Log(entry(1))
	waitFor(t, func() bool { return len(sink.entries()) == 1 })
}

func TestLoggerDropsWhenFullWithoutBlocking(t *testing.T) {
	gate := make(chan struct{})
	sink := &memSink{name: "slow", gate: gate}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}, QueueSize: 2, BatchSize: 1, FlushInterval: time.Hour})

	// The worker takes entries off the queue until the sink and its backlog
	// are busy; after that the queue fills and further entries are dropped.
	start := time.Now()
	accepted := 0
	for i := range 100 {
		if l.Log(entry(i)) {
			accepted++
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Log blocked for %v", elapsed)
	}
	stats := l.Stats()
	if stats.Dropped == 0 || stats.Enqueued != uint64(accepted) || stats.Enqueued+stats.Dropped != 100 {
		t.Fatalf("stats = %+v, accepted = %d", stats, accepted)
	}
	if stats.QueueSize != 2 {
		t.Fatalf("queue size = %d", stats.QueueSize)
	}
	close(gate)
	closeLogger(t, l)
	final := l.Stats()
	written := final.Sinks[0].Written + final.Sinks[0].Dropped
	if written != final.Enqueued {
		t.Fatalf("written+sink dropped = %d, enqueued = %d", written, final.Enqueued)
	}
}

func TestLoggerFlushesOnClose(t *testing.T) {
	sink := &memSink{name: "mem"}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}, BatchSize: 1000, FlushInterval: time.Hour})
	for i := range 50 {
		l.Log(entry(i))
	}
	closeLogger(t, l)
	if got := len(sink.entries()); got != 50 {
		t.Fatalf("flushed %d entries, want 50", got)
	}
	if !sink.closed {
		t.Fatal("sink not closed")
	}
	if l.Log(entry(99)) {
		t.Fatal("Log after Close must drop")
	}
	if l.Stats().Dropped != 1 {
		t.Fatalf("dropped = %d", l.Stats().Dropped)
	}
	// Close is idempotent.
	closeLogger(t, l)
}

func TestLoggerCloseHonoursContext(t *testing.T) {
	gate := make(chan struct{})
	sink := &memSink{name: "stuck", gate: gate}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{sink}, BatchSize: 1, FlushInterval: time.Hour})
	l.Log(entry(1))
	l.Log(entry(2))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := l.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close = %v, want deadline exceeded", err)
	}
	close(gate)
	closeLogger(t, l)
}

func TestLoggerIsolatesFailingSink(t *testing.T) {
	good := &memSink{name: "good"}
	bad := &memSink{name: "bad", err: errors.New("disk full")}
	var mu sync.Mutex
	var reported []string
	l := accesslog.New(accesslog.Options{
		Sinks:         []accesslog.Sink{bad, panicSink{}, good},
		BatchSize:     2,
		FlushInterval: time.Hour,
		OnError: func(sink string, err error) {
			mu.Lock()
			reported = append(reported, sink+": "+err.Error())
			mu.Unlock()
		},
	})
	for i := range 4 {
		l.Log(entry(i))
	}
	closeLogger(t, l)
	if got := len(good.entries()); got != 4 {
		t.Fatalf("good sink got %d entries, want 4", got)
	}
	stats := l.Stats()
	if stats.WriteErrors != 4 {
		t.Fatalf("write errors = %d, want 4 (2 batches x 2 failing sinks)", stats.WriteErrors)
	}
	byName := map[string]accesslog.SinkStats{}
	for _, s := range stats.Sinks {
		byName[s.Name] = s
	}
	if s := byName["bad"]; s.Errors != 2 || s.Failed != 4 || s.LastError != "disk full" {
		t.Fatalf("bad sink stats = %+v", s)
	}
	if s := byName["panic"]; s.Errors != 2 || s.LastError == "" {
		t.Fatalf("panic sink stats = %+v", s)
	}
	if s := byName["good"]; s.Written != 4 || s.Errors != 0 {
		t.Fatalf("good sink stats = %+v", s)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 4 {
		t.Fatalf("OnError calls = %v", reported)
	}
}

type panicSink struct{}

func (panicSink) Name() string                                   { return "panic" }
func (panicSink) Write(context.Context, []accesslog.Entry) error { panic("boom") }
func (panicSink) Close(context.Context) error                    { return nil }

func TestLoggerSlowSinkDoesNotDelayOthers(t *testing.T) {
	gate := make(chan struct{})
	slow := &memSink{name: "slow", gate: gate}
	fast := &memSink{name: "fast"}
	// The backlog of each sink is QueueSize/BatchSize = 8 batches.
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{slow, fast}, BatchSize: 1, FlushInterval: time.Hour, QueueSize: 8})
	for i := range 20 {
		l.Log(entry(i))
		// Let the worker drain the queue so nothing is dropped up front.
		waitFor(t, func() bool { return l.Stats().Queued == 0 })
	}
	waitFor(t, func() bool { return len(fast.entries()) == 20 })
	if d := l.Stats().Sinks[0].Dropped; d == 0 {
		t.Fatal("slow sink should have skipped batches beyond its backlog")
	}
	close(gate)
	closeLogger(t, l)
}

func TestNilLoggerIsDisabled(t *testing.T) {
	var l *accesslog.Logger
	if l.Log(entry(1)) {
		t.Fatal("nil logger accepted an entry")
	}
	if l.Reader() != nil {
		t.Fatal("nil logger has a reader")
	}
	if l.Close(context.Background()) != nil {
		t.Fatal("nil logger close failed")
	}
	if s := l.Stats(); s.QueueSize != 0 || len(s.Sinks) != 0 {
		t.Fatalf("stats = %+v", s)
	}
}

type readerSink struct{ memSink }

func (r *readerSink) Query(context.Context, accesslog.Query) (accesslog.Page, error) {
	return accesslog.Page{}, nil
}

func TestLoggerReaderAndStats(t *testing.T) {
	plain := &memSink{name: "plain"}
	reader := &readerSink{memSink{name: "reader"}}
	l := accesslog.New(accesslog.Options{Sinks: []accesslog.Sink{plain, reader}})
	defer closeLogger(t, l)
	if l.Reader() != accesslog.Reader(reader) {
		t.Fatal("Reader did not return the reader sink")
	}
	stats := l.Stats()
	if len(stats.Sinks) != 2 || stats.Sinks[0].Reader || !stats.Sinks[1].Reader {
		t.Fatalf("sinks = %+v", stats.Sinks)
	}
	if stats.QueueSize != accesslog.DefaultQueueSize {
		t.Fatalf("queue size = %d", stats.QueueSize)
	}
}

var registerTestSinks = sync.OnceFunc(func() {
	accesslog.Register("test-mem", func(_ context.Context, cfg accesslog.Config) (accesslog.Sink, error) {
		return &memSink{name: "test-mem"}, nil
	})
	accesslog.Register("test-fail", func(context.Context, accesslog.Config) (accesslog.Sink, error) {
		return nil, errors.New("no table")
	})
})

func TestRegistryOpen(t *testing.T) {
	registerTestSinks()
	if !accesslog.Registered("test-mem") || accesslog.Registered("nope") {
		t.Fatal("Registered")
	}
	sinks, err := accesslog.Open(context.Background(), accesslog.ParseNames(" test-mem , ,TEST-MEM"), accesslog.Config{})
	if err != nil || len(sinks) != 1 {
		t.Fatalf("Open = %v, %v", sinks, err)
	}
	if _, err := accesslog.Open(context.Background(), []string{"test-mem", "nope"}, accesslog.Config{}); err == nil {
		t.Fatal("unknown sink must fail")
	}
	if _, err := accesslog.Open(context.Background(), []string{"test-fail"}, accesslog.Config{}); err == nil {
		t.Fatal("factory error must fail")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate Register must panic")
		}
	}()
	accesslog.Register("test-mem", func(context.Context, accesslog.Config) (accesslog.Sink, error) { return nil, nil })
}

func TestEntryFailed(t *testing.T) {
	cases := []struct {
		e    accesslog.Entry
		want bool
	}{
		{accesslog.Entry{Code: "ok", Status: 200}, false},
		{accesslog.Entry{Code: "not_found", Status: 404}, true},
		{accesslog.Entry{Code: "internal", Status: 200}, true}, // gRPC errors use HTTP 200
		{accesslog.Entry{Status: 204}, false},
		{accesslog.Entry{Status: 500}, true},
	}
	for _, c := range cases {
		if got := c.e.Failed(); got != c.want {
			t.Errorf("%+v Failed = %v", c.e, got)
		}
	}
}
