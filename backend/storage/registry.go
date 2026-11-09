package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Config selects and configures a driver for Open. Fields that a driver does
// not use are ignored.
type Config struct {
	// Driver is the registered driver name, such as "local" or "s3".
	Driver string
	// PublicBaseURL prefixes keys in Storage.URL. Empty selects the driver
	// default ("/static/" for local; the bucket endpoint for s3).
	PublicBaseURL string

	// LocalDir is the root directory of the local driver.
	LocalDir string
	// SigningSecret signs local presigned uploads.
	SigningSecret []byte

	S3 S3Config
}

// S3Config configures the s3 driver. Credentials come from the standard AWS
// chain (environment, shared config, instance or task role).
type S3Config struct {
	Bucket string
	Region string
	// Endpoint overrides the service endpoint, for MinIO, R2 and other
	// S3-compatible services.
	Endpoint string
	// UsePathStyle addresses buckets as endpoint/bucket/key.
	UsePathStyle bool
}

// Factory builds a Storage from configuration.
type Factory func(ctx context.Context, cfg Config) (Storage, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes a driver available to Open. It panics when name is empty,
// factory is nil or name is already registered; drivers call it from init.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if name == "" || factory == nil {
		panic("storage: Register needs a name and a factory")
	}
	if _, dup := registry[name]; dup {
		panic("storage: driver " + name + " registered twice")
	}
	registry[name] = factory
}

// Registered reports whether a driver named name is available.
func Registered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// Drivers returns the registered driver names in sorted order.
func Drivers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Open builds the driver selected by cfg.Driver.
func Open(ctx context.Context, cfg Config) (Storage, error) {
	registryMu.RLock()
	factory := registry[cfg.Driver]
	registryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("storage: driver %q is not registered (available: %v); import its package, for example storage/s3storage", cfg.Driver, Drivers())
	}
	return factory(ctx, cfg)
}
