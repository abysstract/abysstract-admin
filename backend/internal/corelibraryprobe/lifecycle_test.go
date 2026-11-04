package corelibraryprobe

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func testRuntime() module.Runtime {
	return module.Runtime{
		DB:     &gorm.DB{},
		Logger: slog.New(slog.NewTextHandler(nilWriter{}, nil)),
	}
}

type nilWriter struct{}

func (nilWriter) Write(p []byte) (int, error) { return len(p), nil }

func testDefinition(name string, requires []string, create func() (module.Instance, error)) module.Definition {
	return module.Definition{
		Name:     name,
		Requires: requires,
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return create()
		},
	}
}

func TestValidateBeforeCreate(t *testing.T) {
	valid := testDefinition("valid", nil, func() (module.Instance, error) { return module.Instance{}, nil })
	cases := []struct {
		name        string
		ctx         context.Context
		runtime     module.Runtime
		definitions []module.Definition
		timeout     time.Duration
	}{
		{name: "nil context", ctx: nil, runtime: testRuntime(), definitions: []module.Definition{valid}, timeout: time.Second},
		{name: "nil database", ctx: context.Background(), runtime: module.Runtime{Logger: testRuntime().Logger}, definitions: []module.Definition{valid}, timeout: time.Second},
		{name: "nil logger", ctx: context.Background(), runtime: module.Runtime{DB: testRuntime().DB}, definitions: []module.Definition{valid}, timeout: time.Second},
		{name: "empty name", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{testDefinition(" ", nil, func() (module.Instance, error) { return valid.Create(context.Background(), testRuntime()) })}, timeout: time.Second},
		{name: "duplicate name", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{valid, valid}, timeout: time.Second},
		{name: "dependency not ready", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{testDefinition("later", []string{"first"}, func() (module.Instance, error) { return valid.Create(context.Background(), testRuntime()) })}, timeout: time.Second},
		{name: "nil create", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{{Name: "nil-create"}}, timeout: time.Second},
		{name: "zero timeout", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{valid}, timeout: 0},
		{name: "negative timeout", ctx: context.Background(), runtime: testRuntime(), definitions: []module.Definition{valid}, timeout: -time.Second},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var createCalls atomic.Int32
			for index := range testCase.definitions {
				create := testCase.definitions[index].Create
				if create == nil {
					continue
				}
				testCase.definitions[index].Create = func(ctx context.Context, runtime module.Runtime) (module.Instance, error) {
					createCalls.Add(1)
					return create(ctx, runtime)
				}
			}
			_, err := Start(testCase.ctx, testCase.runtime, testCase.definitions, testCase.timeout)
			if err == nil {
				t.Fatal("Start must reject invalid input")
			}
			if createCalls.Load() != 0 {
				t.Fatalf("Create called %d times before validation completed", createCalls.Load())
			}
		})
	}
}

func TestFirstCreateFails(t *testing.T) {
	createErr := errors.New("first create failed")
	var cleanupCalls atomic.Int32
	_, err := Start(context.Background(), testRuntime(), []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{}, createErr
		}),
		testDefinition("second", []string{"first"}, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { cleanupCalls.Add(1); return nil }}, nil
		}),
	}, time.Second)
	if !errors.Is(err, createErr) || !strings.Contains(err.Error(), `module "first"`) {
		t.Fatalf("error = %v, want named creation error", err)
	}
	if cleanupCalls.Load() != 0 {
		t.Fatal("a failed first creation must not close external resources")
	}
}

func TestLaterCreateFails(t *testing.T) {
	createErr := errors.New("third create failed")
	partialCleanup := atomic.Int32{}
	var orderMu sync.Mutex
	var order []string
	closeModule := func(name string) func(context.Context) error {
		return func(context.Context) error {
			orderMu.Lock()
			order = append(order, name)
			orderMu.Unlock()
			return nil
		}
	}
	_, err := Start(context.Background(), testRuntime(), []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{Close: closeModule("first")}, nil
		}),
		testDefinition("second", []string{"first"}, func() (module.Instance, error) {
			return module.Instance{Close: closeModule("second")}, nil
		}),
		testDefinition("third", []string{"second"}, func() (module.Instance, error) {
			partialCleanup.Add(1)
			return module.Instance{Close: closeModule("third")}, createErr
		}),
	}, time.Second)
	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v, want create error", err)
	}
	if partialCleanup.Load() != 1 {
		t.Fatal("the failing constructor must own its partial-resource cleanup")
	}
	if got, want := strings.Join(order, ","), "second,first"; got != want {
		t.Fatalf("cleanup order = %q, want %q", got, want)
	}
}

func TestCleanupContinues(t *testing.T) {
	createErr := errors.New("create failed")
	secondCloseErr := errors.New("second close failed")
	firstCalled := atomic.Bool{}
	_, err := Start(context.Background(), testRuntime(), []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { firstCalled.Store(true); return nil }}, nil
		}),
		testDefinition("second", []string{"first"}, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { return secondCloseErr }}, nil
		}),
		testDefinition("third", []string{"second"}, func() (module.Instance, error) {
			return module.Instance{}, createErr
		}),
	}, time.Second)
	if !errors.Is(err, createErr) || !errors.Is(err, secondCloseErr) {
		t.Fatalf("error = %v, want creation and cleanup errors", err)
	}
	if !firstCalled.Load() {
		t.Fatal("cleanup must continue after a close error")
	}
}

func TestCloseOnce(t *testing.T) {
	closeErr := errors.New("close failed")
	var calls atomic.Int32
	closer, err := Start(context.Background(), testRuntime(), []module.Definition{
		testDefinition("only", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error { calls.Add(1); return closeErr }}, nil
		}),
	}, time.Second)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	const goroutines = 16
	results := make(chan error, goroutines)
	var wait sync.WaitGroup
	for index := 0; index < goroutines; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- closer(context.Background())
		}()
	}
	wait.Wait()
	close(results)
	var first error
	for result := range results {
		if first == nil {
			first = result
		} else if result != first {
			t.Fatalf("close results are not the same error instance: %p and %p", first, result)
		}
	}
	if calls.Load() != 1 || !errors.Is(first, closeErr) {
		t.Fatalf("close calls=%d error=%v, want one close preserving error", calls.Load(), first)
	}
	if err := closer(nil); err == nil {
		t.Fatal("nil close context must be rejected")
	}
	if calls.Load() != 1 {
		t.Fatal("invalid close call must not consume once-only cleanup")
	}
}

func TestCanceledStartCleanup(t *testing.T) {
	startContext, cancelStart := context.WithCancel(context.Background())
	cancelStart()
	cleanupSawUsableContext := atomic.Bool{}
	createErr := errors.New("create after cancellation")
	_, err := Start(startContext, testRuntime(), []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(ctx context.Context) error {
				if ctx.Err() == nil && ctx.Done() != nil {
					cleanupSawUsableContext.Store(true)
				}
				return nil
			}}, nil
		}),
		testDefinition("second", []string{"first"}, func() (module.Instance, error) {
			return module.Instance{}, createErr
		}),
	}, 100*time.Millisecond)
	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v, want create error", err)
	}
	if !cleanupSawUsableContext.Load() {
		t.Fatal("failed-start cleanup must receive a fresh timeout context")
	}
}

func TestCleanupTimeout(t *testing.T) {
	var secondCalled atomic.Bool
	// The first callback waits for the caller's deadline. The second callback
	// proves cleanup continues with the same context after that deadline.
	closer, err := Start(context.Background(), testRuntime(), []module.Definition{
		testDefinition("first", nil, func() (module.Instance, error) {
			return module.Instance{Close: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}, nil
		}),
		testDefinition("second", []string{"first"}, func() (module.Instance, error) {
			return module.Instance{Close: func(ctx context.Context) error { secondCalled.Store(true); return nil }}, nil
		}),
	}, time.Second)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	closeContext, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := closer(closeContext); err == nil {
		t.Fatal("timed-out close must report the first close error")
	}
	if !secondCalled.Load() {
		t.Fatal("remaining close callbacks must still run")
	}
}

func assertDBQueryWorks(t *testing.T, db *gorm.DB) {
	t.Helper()
	var value int
	if err := db.Raw("SELECT 1").Scan(&value).Error; err != nil {
		t.Fatalf("borrowed DB query failed: %v", err)
	}
	if value != 1 {
		t.Fatalf("borrowed DB query returned %d, want 1", value)
	}
}

func TestDefinitionCompileShape(t *testing.T) {
	definition := module.Definition{
		Name: "compile-shape",
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Services: []module.ConnectService{{Register: func(*connect.Server) {}}}}, nil
		},
	}
	if _, err := Start(context.Background(), testRuntime(), []module.Definition{definition}, time.Second); err != nil {
		t.Fatalf("public definition shape: %v", err)
	}
}
