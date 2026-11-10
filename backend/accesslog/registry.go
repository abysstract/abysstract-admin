package accesslog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Config configures the sinks built by Open. Fields that a sink does not use
// are ignored.
type Config struct {
	// DB is the database of the db sink. It is borrowed and never closed.
	DB *gorm.DB
	// Output is where the stdout sink writes; nil selects os.Stdout.
	Output io.Writer
	// Retention is how long the db sink keeps entries; 0 keeps them forever.
	Retention time.Duration
	// PurgeInterval is how often the db sink deletes expired entries; 0
	// selects one hour.
	PurgeInterval time.Duration
	// Logger receives sink diagnostics, such as failed purges; nil selects
	// slog.Default.
	Logger *slog.Logger
}

// Factory builds a Sink from configuration.
type Factory func(ctx context.Context, cfg Config) (Sink, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes a sink available to Open. It panics when name is empty,
// contains a comma or whitespace, factory is nil or name is already
// registered; sinks call it from init.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if name == "" || factory == nil || strings.ContainsAny(name, ", \t\n") {
		panic("accesslog: Register needs a name without commas or spaces and a factory")
	}
	if _, dup := registry[name]; dup {
		panic("accesslog: sink " + name + " registered twice")
	}
	registry[name] = factory
}

// Registered reports whether a sink named name is available.
func Registered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// Sinks returns the registered sink names in sorted order.
func Sinks() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ParseNames splits a comma-separated sink list such as "db,stdout". Names
// are trimmed and lower-cased; empty items and duplicates are removed.
func ParseNames(list string) []string {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.Split(list, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// Open builds the named sinks in order. When one fails, the sinks already
// built are closed and the error is returned.
func Open(ctx context.Context, names []string, cfg Config) ([]Sink, error) {
	sinks := make([]Sink, 0, len(names))
	for _, name := range names {
		registryMu.RLock()
		factory := registry[name]
		registryMu.RUnlock()
		var err error
		if factory == nil {
			err = fmt.Errorf("accesslog: sink %q is not registered (available: %v); import its package, for example accesslog/dbsink", name, Sinks())
		} else {
			var sink Sink
			sink, err = factory(ctx, cfg)
			if err == nil && sink == nil {
				err = fmt.Errorf("accesslog: sink %q factory returned nil", name)
			}
			if err == nil {
				sinks = append(sinks, sink)
				continue
			}
			err = fmt.Errorf("accesslog: open sink %q: %w", name, err)
		}
		for _, opened := range sinks {
			err = errors.Join(err, opened.Close(ctx))
		}
		return nil, err
	}
	return sinks, nil
}
