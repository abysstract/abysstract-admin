// Package dbsink registers the "db" access log sink. It writes entries to the
// sys_access_log table created by the core migrations, deletes entries older
// than the retention period and implements accesslog.Reader for the admin
// "请求日志" page.
//
// The table is part of the core migrations for PostgreSQL, MySQL, MariaDB and
// SQLite. Applications import their migrations with admin-migrations sync.
package dbsink

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// Name is the registered sink name.
const Name = "db"

// Defaults and limits.
const (
	DefaultPurgeInterval = time.Hour
	DefaultPageSize      = 20
	MaxPageSize          = 500

	// maxAttrsBytes keeps the attrs JSON within a MySQL TEXT column.
	maxAttrsBytes = 60000
	insertBatch   = 200
	purgeBatch    = 5000
)

// ErrNoDB is returned by the factory when Config.DB is nil.
var ErrNoDB = errors.New("dbsink: accesslog.Config.DB is nil")

func init() {
	accesslog.Register(Name, func(ctx context.Context, cfg accesslog.Config) (accesslog.Sink, error) {
		if cfg.DB == nil {
			return nil, ErrNoDB
		}
		if !cfg.DB.Migrator().HasTable(model.TableNameSysAccessLog) {
			return nil, errors.New("dbsink: table sys_access_log is missing; run the migrations (admin-migrations sync imports it)")
		}
		return New(cfg.DB, Options{
			Retention:     cfg.Retention,
			PurgeInterval: cfg.PurgeInterval,
			Logger:        cfg.Logger,
		}), nil
	})
}

// Options configures a Sink.
type Options struct {
	// Retention is how long entries are kept; 0 keeps them forever.
	Retention time.Duration
	// PurgeInterval is how often expired entries are deleted; 0 selects
	// DefaultPurgeInterval. The first purge runs when the sink starts.
	PurgeInterval time.Duration
	// Logger receives purge failures; nil selects slog.Default.
	Logger *slog.Logger
	// Now returns the current time; nil selects time.Now. Tests use it.
	Now func() time.Time
}

// Sink stores entries in sys_access_log.
type Sink struct {
	db     *gorm.DB
	opts   Options
	logger *slog.Logger

	stop      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
}

// New returns a sink on db, which it borrows and never closes. When
// opts.Retention is positive it starts a background purge until Close.
func New(db *gorm.DB, opts Options) *Sink {
	s := &Sink{db: db, opts: opts, logger: opts.Logger, stop: make(chan struct{}), stopped: make(chan struct{})}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if opts.Retention > 0 {
		go s.purgeLoop()
	} else {
		close(s.stopped)
	}
	return s
}

// Name implements accesslog.Namer.
func (s *Sink) Name() string { return Name }

// Write inserts the entries.
func (s *Sink) Write(ctx context.Context, entries []accesslog.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]*model.SysAccessLog, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, toRow(e))
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatch).Error
}

// Close stops the purge loop. The database is not closed.
func (s *Sink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() { close(s.stop) })
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Purge deletes entries that started before cutoff and returns how many were
// deleted. It deletes in batches so large backlogs do not hold long locks.
func (s *Sink) Purge(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		var ids []int64
		err := s.db.WithContext(ctx).Model(&model.SysAccessLog{}).
			Where("log_time < ?", cutoff.UTC()).
			Order("id").Limit(purgeBatch).
			Pluck("id", &ids).Error
		if err != nil || len(ids) == 0 {
			return total, err
		}
		res := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.SysAccessLog{})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if len(ids) < purgeBatch {
			return total, nil
		}
	}
}

func (s *Sink) purgeLoop() {
	defer close(s.stopped)
	interval := s.opts.PurgeInterval
	if interval <= 0 {
		interval = DefaultPurgeInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.purgeOnce()
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
	}
}

func (s *Sink) purgeOnce() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	cutoff := s.now().Add(-s.opts.Retention)
	if _, err := s.Purge(ctx, cutoff); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Warn("access log purge failed", slog.Any("error", err))
	}
}

func (s *Sink) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

// Query implements accesslog.Reader. Entries are ordered newest first.
func (s *Sink) Query(ctx context.Context, q accesslog.Query) (accesslog.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	offset := max(q.Offset, 0)
	db := s.db.WithContext(ctx).Model(&model.SysAccessLog{})
	if !q.From.IsZero() {
		db = db.Where("log_time >= ?", q.From.UTC())
	}
	if !q.To.IsZero() {
		db = db.Where("log_time < ?", q.To.UTC())
	}
	if p := strings.TrimSpace(q.Principal); p != "" {
		db = db.Where("(principal_id = ? OR principal_name = ?)", p, p)
	}
	if t := strings.TrimSpace(q.PrincipalType); t != "" {
		db = db.Where("principal_type = ?", t)
	}
	if p := strings.TrimSpace(q.Procedure); p != "" {
		pattern := escapeLike(p) + "%"
		if !strings.HasPrefix(p, "/") {
			pattern = "%" + pattern
		}
		db = db.Where("(procedure_name LIKE ? ESCAPE '!' OR path LIKE ? ESCAPE '!')", pattern, pattern)
	}
	switch q.Result {
	case accesslog.ResultSuccess:
		db = db.Where("status = ?", statusSuccess)
	case accesslog.ResultFailure:
		db = db.Where("status = ?", statusFailure)
	}
	if c := strings.TrimSpace(q.Code); c != "" {
		db = db.Where("code = ?", c)
	}
	if id := strings.TrimSpace(q.RequestID); id != "" {
		db = db.Where("request_id = ?", id)
	}
	var total int64
	if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return accesslog.Page{}, err
	}
	var rows []*model.SysAccessLog
	if err := db.Order("log_time DESC").Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return accesslog.Page{}, err
	}
	page := accesslog.Page{Total: total, Entries: make([]accesslog.Entry, 0, len(rows))}
	for _, row := range rows {
		page.Entries = append(page.Entries, fromRow(row))
	}
	return page, nil
}

const (
	statusSuccess int16 = 1
	statusFailure int16 = 2
)

func toRow(e accesslog.Entry) *model.SysAccessLog {
	status := statusSuccess
	if e.Failed() {
		status = statusFailure
	}
	return &model.SysAccessLog{
		LogTime:       e.Time.UTC(),
		RequestID:     truncate(e.RequestID, 64),
		Kind:          truncate(e.Kind, 8),
		Service:       truncate(e.Service, 200),
		ProcedureName: truncate(e.Procedure, 255),
		HTTPMethod:    truncate(e.Method, 16),
		Path:          truncate(e.Path, 512),
		PrincipalType: truncate(e.Principal.Type, 32),
		PrincipalID:   truncate(e.Principal.ID, 64),
		PrincipalName: truncate(e.Principal.Name, 128),
		ClientIP:      truncate(e.ClientIP, 64),
		UserAgent:     truncate(e.UserAgent, 255),
		Code:          truncate(e.Code, 32),
		HTTPStatus:    int32(e.Status),
		Status:        status,
		ErrorMsg:      truncate(e.Error, 1000),
		DurationUs:    e.Duration.Microseconds(),
		Attrs:         encodeAttrs(e.Attrs),
	}
}

func fromRow(row *model.SysAccessLog) accesslog.Entry {
	e := accesslog.Entry{
		ID:        strconv.FormatInt(row.ID, 10),
		Time:      row.LogTime.UTC(),
		RequestID: row.RequestID,
		Kind:      row.Kind,
		Service:   row.Service,
		Procedure: row.ProcedureName,
		Method:    row.HTTPMethod,
		Path:      row.Path,
		Principal: accesslog.Principal{
			Type: row.PrincipalType,
			ID:   row.PrincipalID,
			Name: row.PrincipalName,
		},
		ClientIP:  row.ClientIP,
		UserAgent: row.UserAgent,
		Code:      row.Code,
		Status:    int(row.HTTPStatus),
		Error:     row.ErrorMsg,
		Duration:  time.Duration(row.DurationUs) * time.Microsecond,
	}
	if row.Attrs != "" && row.Attrs != "{}" {
		var attrs map[string]any
		if err := json.Unmarshal([]byte(row.Attrs), &attrs); err == nil && len(attrs) > 0 {
			e.Attrs = attrs
		}
	}
	return e
}

func encodeAttrs(attrs map[string]any) string {
	if len(attrs) == 0 {
		return "{}"
	}
	data, err := json.Marshal(attrs)
	if err != nil {
		data, _ = json.Marshal(map[string]any{"_encode_error": err.Error()})
	}
	if len(data) > maxAttrsBytes {
		data, _ = json.Marshal(map[string]any{"_truncated": true, "_bytes": len(data)})
	}
	return string(data)
}

// escapeLike escapes LIKE metacharacters for ESCAPE '!'.
func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

// truncate shortens s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}
