package testdb

import (
	"context"
	"errors"
	"testing"
)

func TestCleanupResourcesReportsAllErrorsAndContinues(t *testing.T) {
	dropErr := errors.New("drop failed")
	closeErr := errors.New("close failed")
	dropCalled := false
	closeCalled := false
	err := cleanupResources(context.Background(), func(context.Context) error {
		dropCalled = true
		return dropErr
	}, func() error {
		closeCalled = true
		return closeErr
	})
	if !dropCalled || !closeCalled {
		t.Fatalf("cleanup calls drop=%v close=%v, want both actions", dropCalled, closeCalled)
	}
	if !errors.Is(err, dropErr) || !errors.Is(err, closeErr) {
		t.Fatalf("cleanup error = %v, want both errors preserved", err)
	}
}

func TestCleanupResourcesRunsCloseWhenSchemaDropIsAbsent(t *testing.T) {
	closeCalled := false
	if err := cleanupResources(context.Background(), nil, func() error {
		closeCalled = true
		return nil
	}); err != nil {
		t.Fatalf("cleanupResources: %v", err)
	}
	if !closeCalled {
		t.Fatal("pool close must run when schema creation did not complete")
	}
}
