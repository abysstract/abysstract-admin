package accesslog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Dispatcher defaults.
const (
	DefaultQueueSize     = 4096
	DefaultBatchSize     = 200
	DefaultFlushInterval = time.Second
	DefaultWriteTimeout  = 10 * time.Second
)

// Options configures a Logger.
type Options struct {
	// Sinks receive every batch. Each sink is written by its own goroutine,
	// so a slow or failing sink does not delay or fail the others.
	Sinks []Sink
	// QueueSize bounds the entries waiting to be written; 0 selects
	// DefaultQueueSize. Entries logged while the queue is full are dropped.
	QueueSize int
	// BatchSize is the largest batch handed to a sink; 0 selects
	// DefaultBatchSize.
	BatchSize int
	// FlushInterval is the longest an entry waits for a batch to fill; 0
	// selects DefaultFlushInterval.
	FlushInterval time.Duration
	// WriteTimeout bounds one Sink.Write call; 0 selects DefaultWriteTimeout.
	WriteTimeout time.Duration
	// OnError is called from the worker when a sink fails. nil logs the
	// failure with Logger.
	OnError func(sink string, err error)
	// Logger receives sink failures when OnError is nil; nil selects
	// slog.Default.
	Logger *slog.Logger
}

// Stats is a snapshot of Logger counters, for health pages and metrics.
type Stats struct {
	// Queued is the number of entries waiting in the queue; QueueSize is its
	// capacity.
	Queued    int
	QueueSize int
	// Enqueued counts accepted entries; Dropped counts entries discarded
	// because the queue was full or the Logger was closed.
	Enqueued uint64
	Dropped  uint64
	// WriteErrors counts failed Sink.Write calls across all sinks.
	WriteErrors uint64
	Sinks       []SinkStats
}

// SinkStats are the counters of one sink.
type SinkStats struct {
	Name string
	// Written counts entries in successful writes; Failed counts entries in
	// failed writes; Dropped counts entries skipped because the sink was
	// still about QueueSize entries behind.
	Written uint64
	Failed  uint64
	Dropped uint64
	Errors  uint64
	// LastError is the message of the most recent failure, or empty.
	LastError string
	// Reader reports whether the sink implements Reader.
	Reader bool
}

type sinkState struct {
	name    string
	sink    Sink
	in      chan []Entry
	written atomic.Uint64
	failed  atomic.Uint64
	dropped atomic.Uint64
	errors  atomic.Uint64
	lastErr atomic.Pointer[string]
}

// Logger is the asynchronous dispatcher between request capture and sinks.
// A nil *Logger is valid and discards everything, so callers need not check
// whether access logging is enabled.
type Logger struct {
	queue    chan Entry
	sinks    []*sinkState
	batch    int
	interval time.Duration
	timeout  time.Duration
	onError  func(string, error)

	mu     sync.RWMutex
	closed bool

	enqueued    atomic.Uint64
	dropped     atomic.Uint64
	writeErrors atomic.Uint64

	done           chan struct{}
	closeOnce      sync.Once
	closeSinksOnce sync.Once
	closeErr       error
}

// New starts a Logger. It owns opts.Sinks and closes them in Close.
func New(opts Options) *Logger {
	l := &Logger{
		queue:    make(chan Entry, positive(opts.QueueSize, DefaultQueueSize)),
		batch:    positive(opts.BatchSize, DefaultBatchSize),
		interval: positiveDuration(opts.FlushInterval, DefaultFlushInterval),
		timeout:  positiveDuration(opts.WriteTimeout, DefaultWriteTimeout),
		onError:  opts.OnError,
		done:     make(chan struct{}),
	}
	if l.onError == nil {
		logger := opts.Logger
		if logger == nil {
			logger = slog.Default()
		}
		l.onError = func(sink string, err error) {
			logger.Warn("access log sink write failed", slog.String("sink", sink), slog.Any("error", err))
		}
	}
	// Each sink may lag behind by about one queue of entries before it
	// loses batches, so bursts that fit the queue reach every healthy sink.
	backlog := max(cap(l.queue)/l.batch, minSinkBacklog)
	for i, sink := range opts.Sinks {
		if sink == nil {
			continue
		}
		name := fmt.Sprintf("sink%d", i)
		if named, ok := sink.(Namer); ok && named.Name() != "" {
			name = named.Name()
		}
		state := &sinkState{name: name, sink: sink, in: make(chan []Entry, backlog)}
		l.sinks = append(l.sinks, state)
	}
	var writers sync.WaitGroup
	for _, state := range l.sinks {
		writers.Add(1)
		go func() {
			defer writers.Done()
			l.runSink(state)
		}()
	}
	go func() {
		l.run()
		for _, state := range l.sinks {
			close(state.in)
		}
		writers.Wait()
		close(l.done)
	}()
	return l
}

// Log enqueues e without blocking. It reports false when the entry was
// dropped because the queue is full or the Logger is closed.
func (l *Logger) Log(e Entry) bool {
	if l == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.closed {
		l.dropped.Add(1)
		return false
	}
	select {
	case l.queue <- e:
		l.enqueued.Add(1)
		return true
	default:
		l.dropped.Add(1)
		return false
	}
}

// Reader returns the first sink that implements Reader, or nil.
func (l *Logger) Reader() Reader {
	if l == nil {
		return nil
	}
	for _, state := range l.sinks {
		if reader, ok := state.sink.(Reader); ok {
			return reader
		}
	}
	return nil
}

// Stats returns a snapshot of the counters. It is safe for concurrent use.
func (l *Logger) Stats() Stats {
	if l == nil {
		return Stats{}
	}
	stats := Stats{
		Queued:      len(l.queue),
		QueueSize:   cap(l.queue),
		Enqueued:    l.enqueued.Load(),
		Dropped:     l.dropped.Load(),
		WriteErrors: l.writeErrors.Load(),
		Sinks:       make([]SinkStats, 0, len(l.sinks)),
	}
	for _, state := range l.sinks {
		item := SinkStats{
			Name:    state.name,
			Written: state.written.Load(),
			Failed:  state.failed.Load(),
			Dropped: state.dropped.Load(),
			Errors:  state.errors.Load(),
		}
		if msg := state.lastErr.Load(); msg != nil {
			item.LastError = *msg
		}
		_, item.Reader = state.sink.(Reader)
		stats.Sinks = append(stats.Sinks, item)
	}
	return stats
}

// Close stops accepting entries, writes the queued entries and closes the
// sinks. When ctx ends first, Close returns its error and the remaining
// entries are written in the background. Close is idempotent.
func (l *Logger) Close(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.queue)
		l.mu.Unlock()
	})
	select {
	case <-l.done:
	case <-ctx.Done():
		return fmt.Errorf("accesslog: flush on close: %w", ctx.Err())
	}
	l.closeSinksOnce.Do(func() {
		var errs []error
		for _, state := range l.sinks {
			if err := state.sink.Close(ctx); err != nil {
				errs = append(errs, fmt.Errorf("accesslog: close sink %q: %w", state.name, err))
			}
		}
		l.closeErr = errors.Join(errs...)
	})
	return l.closeErr
}

// run batches queued entries until the queue is closed and drained.
func (l *Logger) run() {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	batch := make([]Entry, 0, l.batch)
	flush := func(final bool) {
		if len(batch) == 0 {
			return
		}
		for _, state := range l.sinks {
			if final {
				state.in <- batch
				continue
			}
			select {
			case state.in <- batch:
			default:
				state.dropped.Add(uint64(len(batch)))
			}
		}
		batch = make([]Entry, 0, l.batch)
	}
	for {
		select {
		case e, ok := <-l.queue:
			if !ok {
				flush(true)
				return
			}
			batch = append(batch, e)
			if len(batch) >= l.batch {
				flush(false)
			}
		case <-ticker.C:
			flush(false)
		}
	}
}

// minSinkBacklog is the least number of batches that may wait for one sink.
// A sink that falls further behind loses batches (SinkStats.Dropped) instead
// of delaying the other sinks.
const minSinkBacklog = 4

// runSink writes batches to one sink.
func (l *Logger) runSink(state *sinkState) {
	for batch := range state.in {
		err := l.write(state, batch)
		if err == nil {
			state.written.Add(uint64(len(batch)))
			continue
		}
		state.failed.Add(uint64(len(batch)))
		state.errors.Add(1)
		msg := err.Error()
		state.lastErr.Store(&msg)
		l.writeErrors.Add(1)
		l.onError(state.name, err)
	}
}

func (l *Logger) write(state *sinkState, batch []Entry) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("accesslog: sink panicked: %v", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	return state.sink.Write(ctx, batch)
}

func positive(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func positiveDuration(v, fallback time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return fallback
}
