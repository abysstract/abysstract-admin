// Package corelibraryprobe contains executable checks for the public
// module lifecycle contract. It is intentionally internal; the application
// startup implementation is introduced in a later stage.
package corelibraryprobe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/abysstract/abysstract-admin/backend/module"
)

var (
	errNilStartContext   = errors.New("corelibraryprobe: start context is nil")
	errNilRuntimeDB      = errors.New("corelibraryprobe: runtime database is nil")
	errNilRuntimeLog     = errors.New("corelibraryprobe: runtime logger is nil")
	errEmptyModuleName   = errors.New("corelibraryprobe: module name is empty")
	errDuplicateModule   = errors.New("corelibraryprobe: module name is duplicated")
	errMissingDependency = errors.New("corelibraryprobe: module dependency is not ready")
	errNilCreate         = errors.New("corelibraryprobe: module create function is nil")
	errInvalidTimeout    = errors.New("corelibraryprobe: cleanup timeout must be positive")
	errNilCloseContext   = errors.New("corelibraryprobe: close context is nil")
)

type moduleCloser struct {
	name  string
	close func(context.Context) error
}

// Start creates already topologically sorted module definitions and returns a
// once-only reverse-order cleanup function. It validates the complete input
// before invoking any constructor.
func Start(
	ctx context.Context,
	runtime module.Runtime,
	definitions []module.Definition,
	cleanupTimeout time.Duration,
) (close func(context.Context) error, err error) {
	if ctx == nil {
		return nil, errNilStartContext
	}
	if runtime.DB == nil {
		return nil, errNilRuntimeDB
	}
	if runtime.Logger == nil {
		return nil, errNilRuntimeLog
	}
	if cleanupTimeout <= 0 {
		return nil, errInvalidTimeout
	}

	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Name) == "" {
			return nil, fmt.Errorf("%w: %q", errEmptyModuleName, definition.Name)
		}
		if _, exists := seen[definition.Name]; exists {
			return nil, fmt.Errorf("%w: %q", errDuplicateModule, definition.Name)
		}
		if definition.Create == nil {
			return nil, fmt.Errorf("%w: %q", errNilCreate, definition.Name)
		}
		dependencySeen := make(map[string]struct{}, len(definition.Requires))
		for _, dependency := range definition.Requires {
			if strings.TrimSpace(dependency) == "" {
				return nil, fmt.Errorf("%w: %q requires an empty module", errMissingDependency, definition.Name)
			}
			if _, duplicate := dependencySeen[dependency]; duplicate {
				return nil, fmt.Errorf("%w: %q requires %q twice", errMissingDependency, definition.Name, dependency)
			}
			dependencySeen[dependency] = struct{}{}
			if _, ready := seen[dependency]; !ready {
				return nil, fmt.Errorf("%w: %q requires %q", errMissingDependency, definition.Name, dependency)
			}
		}
		seen[definition.Name] = struct{}{}
	}

	created := make([]moduleCloser, 0, len(definitions))
	for _, definition := range definitions {
		instance, createErr := definition.Create(ctx, runtime)
		if createErr != nil {
			cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
			cleanupErr := closeModules(cleanupContext, created)
			cancel()
			creationErr := fmt.Errorf("module %q create: %w", definition.Name, createErr)
			return nil, errors.Join(creationErr, cleanupErr)
		}
		if instance.Close != nil {
			created = append(created, moduleCloser{name: definition.Name, close: instance.Close})
		}
	}

	var once sync.Once
	var closeErr error
	return func(closeContext context.Context) error {
		if closeContext == nil {
			return errNilCloseContext
		}
		once.Do(func() {
			closeErr = closeModules(closeContext, created)
		})
		return closeErr
	}, nil
}

func closeModules(ctx context.Context, modules []moduleCloser) error {
	var closeErrors []error
	for index := len(modules) - 1; index >= 0; index-- {
		entry := modules[index]
		if err := entry.close(ctx); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("module %q close: %w", entry.name, err))
		}
	}
	return errors.Join(closeErrors...)
}
